package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/rely"
	cfg "github.com/vertex-lab/relay/pkg/config"
	"github.com/vertex-lab/relay/pkg/credits"
	"github.com/vertex-lab/relay/pkg/dvm"
)

func SetupRelay(config cfg.RelayConfig) *rely.Relay {
	relay := rely.NewRelay(
		rely.WithDomain("vertexlab.io"),
		rely.WithQueueCapacity(config.QueueCapacity),
		rely.WithMaxProcessors(config.Processors),
	)

	dvm := dvm.Handler{
		Service:   service,
		SecretKey: config.SecretKey,
	}

	relay.Reject.Event = append(relay.Reject.Event, UnsupportedDVM)
	relay.Reject.Req = append(relay.Reject.Req, FiltersExceed(50), WithSearch, UnauthedCredits)
	relay.Reject.Count = append(relay.Reject.Count, FiltersExceed(100))
	relay.On.Connect = func(c rely.Client) { c.SendAuth() }
	relay.On.Req = Query
	relay.On.Count = Count
	relay.On.Event = Process(dvm)
	return relay
}

func Query(ctx context.Context, client rely.Client, filters nostr.Filters) ([]nostr.Event, error) {
	events, err := query(ctx, client, filters)
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("failed to query", "filters", filters, "error", err)
		return nil, err
	}
	return events, err
}

func query(ctx context.Context, client rely.Client, filters nostr.Filters) ([]nostr.Event, error) {
	events, err := service.Sqlite.Query(ctx, filters...)
	if err != nil {
		return nil, err
	}

	if ContainCreditQuery(filters) {
		credits, err := creditQuery(client.Pubkey())
		if err != nil {
			return nil, err
		}
		events = append(events, credits)
	}
	return events, nil
}

func creditQuery(pubkey string) (nostr.Event, error) {
	bucket, err := service.Credits.Bucket(pubkey)
	if err != nil {
		return nostr.Event{}, fmt.Errorf("failed to query credits of pubkey %s: %w", pubkey, err)
	}

	credits, err := CreditEvent(bucket)
	if err != nil {
		return nostr.Event{}, fmt.Errorf("failed to query credits of pubkey %s: %w", pubkey, err)
	}
	return credits, nil
}

func Count(client rely.Client, filters nostr.Filters) (count int64, approx bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	count, err = service.Sqlite.Count(ctx, filters...)
	if err != nil {
		return 0, false, err
	}
	return count, false, nil
}

func Process(dvm dvm.Handler) func(rely.Client, *nostr.Event) error {
	return func(client rely.Client, request *nostr.Event) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		response := dvm.Process(ctx, request)
		_, err := service.Sqlite.Save(ctx, response)
		return err
	}
}

func UnsupportedDVM(_ rely.Client, event *nostr.Event) error {
	if !dvm.Supports(event.Kind) {
		return fmt.Errorf("%w: %d", dvm.ErrUnsupportedKind, event.Kind)
	}
	return nil
}

func FiltersExceed(n int) func(rely.Client, nostr.Filters) error {
	return func(_ rely.Client, filters nostr.Filters) error {
		if len(filters) > n {
			return fmt.Errorf("number of filters exceed the maximum allowed (%d): %d", n, len(filters))
		}
		return nil
	}
}

func WithSearch(_ rely.Client, filters nostr.Filters) error {
	for _, f := range filters {
		if f.Search != "" {
			return errors.New("NIP-50 search is not supported")
		}
	}
	return nil
}

func UnauthedCredits(client rely.Client, filters nostr.Filters) error {
	if ContainCreditQuery(filters) && client.Pubkey() == "" {
		return errors.New("auth-required: you must be authenticated to request your credit balance")
	}
	return nil
}

func ContainCreditQuery(filters nostr.Filters) bool {
	for _, f := range filters {
		if slices.Contains(f.Kinds, 22243) {
			return true
		}
	}
	return false
}

// CreditEvent returns the [credits.Bucket] as a signed kind 22243 nostr event
func CreditEvent(b credits.Bucket) (nostr.Event, error) {
	event := nostr.Event{
		Kind:      22243,
		CreatedAt: nostr.Now(),
		Tags: nostr.Tags{
			{"credits", strconv.Itoa(b.Tokens)},
			{"lastRequest", strconv.FormatInt(b.LastModified, 10)},
		},
	}

	err := event.Sign(config.Relay.SecretKey)
	if err != nil {
		return nostr.Event{}, fmt.Errorf("failed to sign credit event: %w", err)
	}
	return event, nil
}
