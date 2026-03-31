package relay

import (
	"cmp"
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

func Setup(
	config Config,
	service *core.Service,
	limiter rate.Limiter,
) *rely.Relay {

	info := nip11.RelayInformationDocument{
		Name:          "Vertex Relay",
		Description:   "Web of Trust Relay powered by Vertex",
		PubKey:        config.PublicKey,
		SupportedNIPs: []any{1, 11, 42, 45, 50},
		Software:      "https://github.com/vertex-lab/relay",
		Icon:          "https://image.nostr.build/7afc9d727d6486851cc2fe09865e7cc383449f8bad1700a9508db4d2815b6f1a.png",
	}

	relay := rely.NewRelay(
		rely.WithAuthURL(config.Domain),
		rely.WithQueueCapacity(config.QueueCapacity),
		rely.WithMaxProcessors(config.Processors),
		rely.WithInfo(info),
	)

	h := handler{
		service:   service,
		relay:     relay,
		limiter:   limiter,
		secretKey: config.SecretKey,
	}

	relay.Reject.Connection.Clear()
	relay.Reject.Connection.Append(
		rely.RegistrationFailWithin(3*time.Second),
		RateConnIP(limiter, 1),
	)

	relay.Reject.Event.Clear()
	relay.Reject.Event.Append(
		RateEventIP(limiter, 5),
		rely.InvalidID,
		rely.InvalidSignature,
		UnsupportedDVM,
	)

	relay.Reject.Req.Clear()
	relay.Reject.Req.Append(
		RateFiltersIP(limiter, 1),
		FiltersExceed(50),
		InvalidSearch,
		UnauthedCredits,
	)

	relay.Reject.Count.Clear()
	relay.Reject.Count.Append(
		RateFiltersIP(limiter, 1),
		FiltersExceed(100),
	)

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

	if _, err := h.service.Sqlite.Save(ctx, response); err != nil {
		slog.Error("failed to save dvm response", "error", err)
		return err
	}

	h.stats.Record(statsDVM)
	return nil
}

func (h *handler) Query(ctx context.Context, client rely.Client, id string, filters nostr.Filters) ([]nostr.Event, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, err := h.query(ctx, client, filters)
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("failed to query", "filters", filters, "error", err)
		return nil, err
	}

	h.stats.Record(statsREQ)
	return events, err
}

func (h *handler) query(ctx context.Context, client rely.Client, filters nostr.Filters) ([]nostr.Event, error) {
	if IsSearchQuery(filters) {
		events, err := h.searchQuery(ctx, filters[0])
		if err != nil {
			return nil, err
		}

		h.stats.Record(statsSearch)
		return events, nil
	}

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

func (h *handler) searchQuery(ctx context.Context, filter nostr.Filter) ([]nostr.Event, error) {
	if filter.LimitZero {
		return nil, nil
	}
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if filter.Limit > 20 {
		filter.Limit = 20
	}

	args := core.SearchProfilesArgs{
		Algorithm: core.Algorithm{Sort: core.Global},
		Search:    filter.Search,
		Limit:     filter.Limit,
	}
	if err := args.Normalize(); err != nil {
		return nil, err
	}

	search, err := h.service.SearchProfiles(ctx, args)
	if err != nil {
		return nil, err
	}

	pubkeys := make([]string, 0, len(search.Results))
	ranks := make(map[string]float64, len(search.Results))
	for _, p := range search.Results {
		pubkeys = append(pubkeys, p.Pubkey)
		ranks[p.Pubkey] = p.Rank
	}

	filter.Authors = pubkeys
	filter.Search = ""

	profiles, err := h.service.Sqlite.Query(ctx, filter)
	if err != nil {
		return nil, err
	}

	slices.SortFunc(profiles, func(e1, e2 nostr.Event) int {
		return cmp.Compare(ranks[e1.PubKey], ranks[e2.PubKey])
	})
	return profiles, nil
}

func (h *handler) Count(client rely.Client, id string, filters nostr.Filters) (count int64, approx bool, err error) {
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

func RateConnIP(l rate.Limiter, cost float64) func(rely.Stats, *http.Request) error {
	return func(_ rely.Stats, r *http.Request) error {
		ip := rely.GetIP(r).Group()
		if !l.Allow(ip, cost) {
			return ErrIPRateLimited
		}
		return nil
	}
}

func RateFiltersIP(l rate.Limiter, cost float64) func(c rely.Client, _ string, f nostr.Filters) error {
	return func(c rely.Client, _ string, f nostr.Filters) error {
		ip := c.IP().Group()
		if !l.Allow(ip, cost*float64(len(f))) {
			c.Disconnect()
			return ErrIPRateLimited
		}
		return nil
	}
}

func RateEventIP(l rate.Limiter, cost float64) func(c rely.Client, _ *nostr.Event) error {
	return func(c rely.Client, _ *nostr.Event) error {
		ip := c.IP().Group()
		if !l.Allow(ip, cost) {
			c.Disconnect()
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

func FiltersExceed(n int) func(c rely.Client, id string, filters nostr.Filters) error {
	return func(_ rely.Client, _ string, filters nostr.Filters) error {
		if len(filters) > n {
			return fmt.Errorf("number of filters exceed the maximum allowed (%d): %d", n, len(filters))
		}
		return nil
	}
}

func InvalidSearch(_ rely.Client, _ string, filters nostr.Filters) error {
	searches := 0
	for _, f := range filters {
		if f.Search != "" {
			searches++
		}
	}

	if searches == 0 {
		return nil
	}
	if len(filters) != 1 {
		return errors.New("only one filter is allowed for search queries")
	}
	if !slices.Equal(filters[0].Kinds, []int{nostr.KindProfileMetadata}) {
		return errors.New("we support only kind:0 search queries")
	}
	if len(filters[0].Authors) > 0 {
		return errors.New("we don't support authors in kind:0 search queries")
	}
	if len(filters[0].Search) < 3 {
		return errors.New("search query must be at least 3 characters")
	}
	return nil
}

func IsSearchQuery(filters nostr.Filters) bool {
	if len(filters) != 1 {
		return false
	}
	return filters[0].Search != ""
}

func UnauthedCredits(client rely.Client, id string, filters nostr.Filters) error {
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
