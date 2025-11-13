package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/pippellia-btc/rely"
	"github.com/vertex-lab/crawler_v2/pkg/regraph"
	nstore "github.com/vertex-lab/crawler_v2/pkg/store"
	sqlite "github.com/vertex-lab/nostr-sqlite"
	cfg "github.com/vertex-lab/relay/pkg/config"
	"github.com/vertex-lab/relay/pkg/dvm"
	"github.com/vertex-lab/relay/pkg/rate"

	"github.com/nbd-wtf/go-nostr"
	"github.com/redis/go-redis/v9"
)

var (
	relay  *rely.Relay
	config cfg.Config
	err    error

	store *sqlite.Store
	db    regraph.DB

	limiter   rate.Limiter
	processed atomic.Int32
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go rely.HandleSignals(cancel)

	// removing go-nostr logs
	nostr.DebugLogger.SetOutput(io.Discard)
	nostr.InfoLogger.SetOutput(io.Discard)

	slog.Info("--------- starting up the relay --------")
	defer slog.Info("-----------------------------------------")

	config, err = cfg.Load()
	if err != nil {
		panic(err)
	}

	relay = rely.NewRelay(
		rely.WithDomain("vertexlab.io"),
		rely.WithQueueCapacity(config.QueueCapacity),
		rely.WithMaxProcessors(config.Processors),
	)

	store, err = nstore.New(config.SQLiteURL)
	if err != nil {
		panic(err)
	}

	defer store.Close()
	slog.Info("sqlite connected", "address", config.SQLiteURL)

	db, err = regraph.New(&redis.Options{Addr: config.RedisAddress})
	if err != nil {
		panic(err)
	}

	defer db.Close()
	slog.Info("redis connected", "address", config.RedisAddress)

	limiter, err = rate.NewLimiter(db.Client, config.Refill)
	if err != nil {
		panic(err)
	}

	relay.Reject.Event = append(relay.Reject.Event, NonDVM)
	relay.Reject.Req = append(relay.Reject.Req, FiltersExceed(100), WithSearch, UnauthedCredits)
	relay.On.Connect = SendAuth
	relay.On.Req = Query
	relay.On.Count = Count
	relay.On.Event = Process

	err := relay.StartAndServe(ctx, config.RelayAddress)
	if err != nil {
		panic(err)
	}
}

// Query the event store, or redis for the credit balance, and log every error.
func Query(ctx context.Context, client rely.Client, filters nostr.Filters) ([]nostr.Event, error) {
	events, err := query(ctx, client, filters)
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("failed to query", "filters", filters, "error", err)
		return nil, err
	}
	return events, err
}

func query(ctx context.Context, client rely.Client, filters nostr.Filters) ([]nostr.Event, error) {
	events, err := store.Query(ctx, filters...)
	if err != nil {
		return nil, err
	}

	if ContainCreditQuery(filters) {
		info, err := creditQuery(client.Pubkey())
		if err != nil {
			return nil, err
		}
		events = append(events, info)
	}
	return events, nil
}

func creditQuery(pubkey string) (nostr.Event, error) {
	if pubkey == "" {
		return nostr.Event{}, errors.New("failed to query credits: pubkey is empty")
	}

	bucket, err := limiter.Bucket(pubkey)
	if err != nil {
		return nostr.Event{}, fmt.Errorf("failed to query credits of pubkey %s: %w", pubkey, err)
	}

	info := bucket.ToEvent()
	if err = info.Sign(config.SecretKey); err != nil {
		return nostr.Event{}, fmt.Errorf("failed to query credits: failed to sign: %w", err)
	}
	return info, nil
}

func Count(client rely.Client, filters nostr.Filters) (count int64, approx bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	count, err = store.Count(ctx, filters...)
	if err != nil {
		return 0, false, err
	}
	return count, false, nil
}

func Process(_ rely.Client, event *nostr.Event) error {
	err := process(event, SignAndSave)
	if err != nil {
		slog.Error("failed to process request", "event", event, "error", err)
	}
	return err
}

func SignAndSave(e *nostr.Event) error {
	if err := e.Sign(config.SecretKey); err != nil {
		return fmt.Errorf("failed to sign the response %s: %w", e.ID, err)
	}

	relay.Broadcast(e)
	_, err := store.Save(context.Background(), e)
	return err
}

func process(event *nostr.Event, reply func(*nostr.Event) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	request, err := dvm.Parse(event)
	if err != nil {
		return reply(dvm.ErrorEvent(err, dvm.NewRecord(event)))
	}

	if !limiter.Allow(request.Pubkey, cost(request)) {
		return reply(dvm.ErrorEvent(dvm.ErrNoCredits, request.Record))
	}

	request.Nodes, err = db.NodeCount(ctx)
	if err != nil {
		return reply(dvm.ErrorEvent(err, request.Record))
	}

	var response dvm.Response
	switch request.Kind {
	case dvm.KindVerifyReputation:
		response, err = dvm.VerifyReputation(ctx, db, request)

	case dvm.KindRecommendFollows:
		response, err = dvm.RecommendFollows(ctx, db, request)

	case dvm.KindRankProfiles:
		response, err = dvm.RankProfiles(ctx, db, request)

	case dvm.KindSearchProfiles:
		response, err = dvm.SearchProfiles(ctx, db, store, request)

	default:
		err = fmt.Errorf("%w: %d", dvm.ErrUnsupportedKind, request.Kind)
	}

	if err != nil {
		return reply(dvm.ErrorEvent(err, request.Record))
	}

	tot := processed.Add(1)
	if tot%1000 == 0 {
		log.Printf("processed %d dvm requests", tot)
	}

	return reply(dvm.ResponseEvent(response, request))
}

// This function estimates the cost of processing a request with the provided params.
func cost(r *dvm.Request) int {
	switch r.Sort {
	case dvm.Personalized:
		return 10

	default:
		return 1
	}
}
