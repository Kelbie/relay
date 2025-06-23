package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"sync/atomic"
	"time"

	. "github.com/pippellia-btc/rely"
	"github.com/vertex-lab/crawler_v2/pkg/redb"
	cfg "github.com/vertex-lab/relay/pkg/config"
	"github.com/vertex-lab/relay/pkg/dvm"
	"github.com/vertex-lab/relay/pkg/eventstore"
	"github.com/vertex-lab/relay/pkg/rate"

	"github.com/nbd-wtf/go-nostr"
	"github.com/redis/go-redis/v9"
)

var (
	ErrUnathedCreditsQuery = errors.New("auth-required: you must be authenticated to request your credit balance")
)

var (
	relay  *Relay
	config *cfg.Config
	err    error

	store *eventstore.Store
	db    redb.RedisDB

	limiter   rate.Limiter
	processed atomic.Int32
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go HandleSignals(cancel)

	config, err = cfg.Load()
	if err != nil {
		panic(err)
	}

	nostr.DebugLogger.SetOutput(os.Stdout)
	nostr.InfoLogger.SetOutput(io.Discard) // discarding info logs

	log.Println("starting up the relay")
	defer log.Println("shutdown")

	relay = NewRelay(
		WithDomain("vertexlab.io"),
		WithOverloadLogs(),
		WithQueueCapacity(2000),
		WithMaxProcessors(10),
	)

	store, err = eventstore.New(config.SQLitePath)
	if err != nil {
		panic(err)
	}
	log.Printf("sqlite connected to %s", config.SQLitePath)

	db = redb.New(&redis.Options{Addr: config.RedisAddress})
	limiter = rate.NewLimiterWithPolicy(db.Client, config.Limits)
	log.Printf("redis connected at %s", config.RedisAddress)

	relay.RejectEvent = append(relay.RejectEvent, NonDVMs)
	relay.RejectReq = append(relay.RejectReq, UnauthedCredits)
	relay.OnEvent = Process
	relay.OnReq = Query

	log.Printf("relay running at %s", config.RelayAddress)
	if err := relay.StartAndServe(ctx, config.RelayAddress); err != nil {
		panic(err)
	}
}

func NonDVMs(_ Client, event *nostr.Event) error {
	if event.Kind < 5312 || event.Kind > 5315 {
		return fmt.Errorf("%w: %d", dvm.ErrUnsupportedKind, event.Kind)
	}
	return nil
}

func UnauthedCredits(client Client, filters nostr.Filters) error {
	for _, filter := range filters {
		if ContainsCreditQuery(filter) && client.Pubkey() == "" {
			client.SendAuthChallenge()
			return ErrUnathedCreditsQuery
		}
	}
	return nil
}

// Query the event store, or redis for the credit balance, and log every error.
func Query(ctx context.Context, client Client, filters nostr.Filters) ([]nostr.Event, error) {
	events := make([]nostr.Event, 0, len(filters))
	for _, filter := range filters {

		if ContainsCreditQuery(filter) {
			bucket, err := limiter.Bucket(client.Pubkey())
			if err != nil {
				log.Println(err)
				return nil, err
			}

			info := bucket.ToEvent()
			if err := info.Sign(config.Secret); err != nil {
				log.Printf("failed to sign credit event for %s: %v", client.Pubkey(), err)
				return nil, fmt.Errorf("failed to sign credit event: %w", err)
			}

			events = append(events, info)
		}

		found, err := store.Query(ctx, &filter)
		if err != nil {
			log.Printf("failed to query for %v: %v", filter, err)
			return nil, fmt.Errorf("failed to query for %v: %w", filter, err)
		}

		events = append(events, found...)

	}

	return events, nil
}

func ContainsCreditQuery(filter nostr.Filter) bool {
	return slices.Contains(filter.Kinds, 22243)
}

func Process(_ Client, event *nostr.Event) error {
	err := process(event, func(event *nostr.Event) error {
		if err := event.Sign(config.Secret); err != nil {
			return fmt.Errorf("failed to sign the response %s: %w", event.ID, err)
		}

		relay.Broadcast(event)
		return store.Save(context.Background(), event)
	})

	if err != nil {
		log.Printf("failed to process request: %v", err)
	}
	return err
}

func process(event *nostr.Event, reply func(*nostr.Event) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	request, err := dvm.Parse(event)
	if err != nil {
		return reply(dvm.ErrorEvent(err, dvm.NewRecord(event)))
	}

	paid := limiter.Allow(request.Pubkey, cost(request))
	if !paid {
		return reply(dvm.ErrorEvent(dvm.ErrNoCredits, request.Record))
	}

	request.Nodes, err = db.NodeCount(ctx)
	if err != nil {
		return err
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
