package main

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/rely"
	"github.com/vertex-lab/relay/pkg/credits"
	"github.com/vertex-lab/relay/pkg/dvm"
)

func SendAuth(c rely.Client) { c.SendAuth() }

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

// CreditEvent returns the [rate.Bucket] as a signed kind 22243 nostr event
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
