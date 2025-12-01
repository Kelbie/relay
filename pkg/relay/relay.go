package relay

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/rely"
	"github.com/vertex-lab/relay/pkg/core"
	"github.com/vertex-lab/relay/pkg/dvm"
	"github.com/vertex-lab/relay/pkg/rate"
)

var (
	ErrIPRateLimited = errors.New("rate-limited: slow down there chief")
)

type handler struct {
	service   *core.Service
	relay     *rely.Relay
	limiter   *rate.Limiter
	dvm       dvm.Handler
	secretKey string
}

func Setup(config Config, service *core.Service, limiter *rate.Limiter) *rely.Relay {
	relay := rely.NewRelay(
		rely.WithDomain(config.Domain),
		rely.WithQueueCapacity(config.QueueCapacity),
		rely.WithMaxProcessors(config.Processors),
	)

	h := handler{
		service:   service,
		relay:     relay,
		dvm:       dvm.Handler{Service: service, SecretKey: config.SecretKey},
		limiter:   limiter,
		secretKey: config.SecretKey,
	}

	relay.Reject.Connection = []func(rely.Stats, *http.Request) error{h.ConnRateLimit(0.1), rely.RegistrationFailWithin(3 * time.Second)}
	relay.Reject.Event = []func(rely.Client, *nostr.Event) error{h.EventRateLimit(1), UnsupportedDVM, rely.InvalidID, rely.InvalidSignature}
	relay.Reject.Req = []func(rely.Client, nostr.Filters) error{h.QueryRateLimit(0.1), FiltersExceed(50), WithSearch, UnauthedCredits}
	relay.Reject.Count = []func(rely.Client, nostr.Filters) error{h.QueryRateLimit(0.1), FiltersExceed(100)}

	relay.On.Connect = func(c rely.Client) { c.SendAuth() }
	relay.On.Req = h.Query
	relay.On.Count = h.Count
	relay.On.Event = h.Process
	return relay
}

func (h handler) Process(_ rely.Client, request *nostr.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response := h.dvm.Process(ctx, request)
	h.relay.Broadcast(response)
	_, err := h.service.Sqlite.Save(ctx, response)
	return err
}

func (h handler) Query(ctx context.Context, client rely.Client, filters nostr.Filters) ([]nostr.Event, error) {
	events, err := h.query(ctx, client, filters)
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("failed to query", "filters", filters, "error", err)
		return nil, err
	}
	return events, err
}

func (h handler) query(ctx context.Context, client rely.Client, filters nostr.Filters) ([]nostr.Event, error) {
	events, err := h.service.Sqlite.Query(ctx, filters...)
	if err != nil {
		return nil, err
	}

	if ContainCreditQuery(filters) {
		credits, err := h.creditQuery(client.Pubkeys()...)
		if err != nil {
			return nil, err
		}
		events = append(events, credits...)
	}
	return events, nil
}

func (h handler) creditQuery(pubkeys ...string) ([]nostr.Event, error) {
	if len(pubkeys) == 0 {
		return nil, nil
	}

	events := make([]nostr.Event, 0, len(pubkeys))
	for _, pk := range pubkeys {

		bucket, err := h.service.Credits.Bucket(pk)
		if err != nil {
			return nil, fmt.Errorf("failed to query credits of pubkey %s: %w", pk, err)
		}

		event := bucket.ToEvent()
		err = event.Sign(h.secretKey)
		if err != nil {
			return nil, fmt.Errorf("failed to sign credit event: %w", err)
		}

		events = append(events, event)
	}
	return events, nil
}

func (h handler) Count(client rely.Client, filters nostr.Filters) (count int64, approx bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	count, err = h.service.Sqlite.Count(ctx, filters...)
	if err != nil {
		return 0, false, err
	}
	return count, false, nil
}

func (h handler) ConnRateLimit(cost float32) func(rely.Stats, *http.Request) error {
	return func(_ rely.Stats, r *http.Request) error {
		ip := rely.GetIP(r).Group()
		if h.limiter.Reject(ip, cost) {
			return ErrIPRateLimited
		}
		return nil
	}
}

func (h handler) QueryRateLimit(baseCost float32) func(rely.Client, nostr.Filters) error {
	return func(c rely.Client, f nostr.Filters) error {
		ip := c.IP().Group()
		cost := baseCost * float32(len(f))
		if h.limiter.Reject(ip, cost) {
			defer c.Disconnect()
			return ErrIPRateLimited
		}
		return nil
	}
}

func (h handler) EventRateLimit(cost float32) func(rely.Client, *nostr.Event) error {
	return func(c rely.Client, _ *nostr.Event) error {
		ip := c.IP().Group()
		if h.limiter.Reject(ip, cost) {
			defer c.Disconnect()
			return ErrIPRateLimited
		}
		return nil
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
	if ContainCreditQuery(filters) && !client.IsAuthed() {
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
