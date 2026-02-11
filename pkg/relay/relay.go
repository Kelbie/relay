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
	"github.com/nbd-wtf/go-nostr/nip11"
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
	limiter   rate.Limiter
	secretKey string
	stats
}

func Setup(config Config, service *core.Service, limiter rate.Limiter) *rely.Relay {

	info := nip11.RelayInformationDocument{
		Name:          "Vertex Relay",
		Description:   "DVM Web of Trust Relay powered by Vertex",
		PubKey:        config.PublicKey,
		SupportedNIPs: []any{1, 11, 42, 45},
		Software:      "https://github.com/vertex-lab/relay",
		Icon:          "https://image.nostr.build/7afc9d727d6486851cc2fe09865e7cc383449f8bad1700a9508db4d2815b6f1a.png",
	}

	relay := rely.NewRelay(
		rely.WithDomain(config.Domain),
		rely.WithQueueCapacity(config.QueueCapacity),
		rely.WithMaxProcessors(config.Processors),
		rely.WithInfo(info),
	)

	h := handler{
		service:   service,
		relay:     relay,
		limiter:   limiter,
		secretKey: config.SecretKey,
		stats:     stats{logEvery: config.LogEvery},
	}

	relay.Reject.Connection.Prepend(h.CostPerConn(1))
	relay.Reject.Event.Prepend(h.CostPerEvent(1), UnsupportedDVM)
	relay.Reject.Req.Prepend(h.CostPerFilter(0.1), FiltersExceed(50), WithSearch, UnauthedCredits)
	relay.Reject.Count.Prepend(h.CostPerFilter(0.1), FiltersExceed(100))

	relay.On.Connect = func(c rely.Client) { c.SendAuth() }
	relay.On.Req = h.Query
	relay.On.Count = h.Count
	relay.On.Event = h.Process
	return relay
}

func (h *handler) Process(_ rely.Client, request *nostr.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response := dvm.Handler{Service: h.service, SecretKey: h.secretKey}.Process(ctx, request)
	if err := h.relay.Broadcast(response); err != nil {
		slog.Error("failed to broadcast dvm response", "error", err)
	}

	_, err := h.service.Sqlite.Save(ctx, response)
	if err != nil {
		slog.Error("failed to save dvm response", "error", err)
		return err
	}

	h.stats.Record(statsDVM)
	return nil
}

func (h *handler) Query(ctx context.Context, client rely.Client, filters nostr.Filters) ([]nostr.Event, error) {
	events, err := h.query(ctx, client, filters)
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("failed to query", "filters", filters, "error", err)
		return nil, err
	}

	h.stats.Record(statsREQ)
	return events, err
}

func (h *handler) query(ctx context.Context, client rely.Client, filters nostr.Filters) ([]nostr.Event, error) {
	events, err := h.service.Sqlite.Query(ctx, filters...)
	if err != nil {
		return nil, err
	}

	if ContainCreditQuery(filters) {
		credits, err := h.creditQuery(client.Pubkeys()...)
		if err != nil {
			return nil, err
		}

		h.stats.Record(statsCredit)
		events = append(events, credits...)
	}
	return events, nil
}

func (h *handler) creditQuery(pubkeys ...string) ([]nostr.Event, error) {
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

func (h *handler) Count(client rely.Client, filters nostr.Filters) (count int64, approx bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	count, err = h.service.Sqlite.Count(ctx, filters...)
	if err != nil {
		slog.Error("failed to count", "filters", filters, "error", err)
		return 0, false, err
	}

	h.stats.Record(statsCOUNT)
	return count, false, nil
}

func (h *handler) CostPerConn(cost float64) func(rely.Stats, *http.Request) error {
	return func(_ rely.Stats, r *http.Request) error {
		ip := rely.GetIP(r).Group()
		if !h.limiter.Allow(ip, cost) {
			return ErrIPRateLimited
		}
		return nil
	}
}

func (h *handler) CostPerFilter(cost float64) func(rely.Client, nostr.Filters) error {
	return func(c rely.Client, f nostr.Filters) error {
		cost = cost * float64(len(f))
		ip := c.IP().Group()
		if !h.limiter.Allow(ip, cost) {
			defer c.Disconnect()
			return ErrIPRateLimited
		}
		return nil
	}
}

func (h *handler) CostPerEvent(cost float64) func(rely.Client, *nostr.Event) error {
	return func(c rely.Client, _ *nostr.Event) error {
		ip := c.IP().Group()
		if !h.limiter.Allow(ip, cost) {
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
