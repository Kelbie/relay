package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"github.com/pippellia-btc/rely"
	cfg "github.com/vertex-lab/relay/pkg/config"
	"github.com/vertex-lab/relay/pkg/dvm"
	"github.com/vertex-lab/relay/pkg/rate"
	srv "github.com/vertex-lab/relay/pkg/service"

	"github.com/nbd-wtf/go-nostr"
)

var (
	config cfg.Config
	err    error

	service    *srv.Service
	limiter    rate.Limiter
	dvmHandler dvm.Handler
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// removing go-nostr logs
	nostr.DebugLogger.SetOutput(io.Discard)
	nostr.InfoLogger.SetOutput(io.Discard)

	slog.Info("--------- starting up the relay --------")
	defer slog.Info("-----------------------------------------")

	config, err = cfg.Load()
	if err != nil {
		panic(err)
	}

	service, err = srv.New(config.Service)
	if err != nil {
		panic(err)
	}
	defer service.Close()

	limiter, err = rate.NewLimiter(service.Redis.Client, config.Refill)
	if err != nil {
		panic(err)
	}

	dvmHandler = dvm.Handler{
		Service:   service,
		Limiter:   limiter,
		SecretKey: config.Relay.SecretKey,
	}

	relay := rely.NewRelay(
		rely.WithDomain("vertexlab.io"),
		rely.WithQueueCapacity(config.Relay.QueueCapacity),
		rely.WithMaxProcessors(config.Relay.Processors),
	)

	relay.Reject.Event = append(relay.Reject.Event, UnsupportedDVM)
	relay.Reject.Req = append(relay.Reject.Req, FiltersExceed(100), WithSearch, UnauthedCredits)
	relay.Reject.Count = append(relay.Reject.Count, FiltersExceed(100))
	relay.On.Connect = SendAuth
	relay.On.Req = Query
	relay.On.Count = Count
	relay.On.Event = Process

	err := relay.StartAndServe(ctx, config.Relay.Address)
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
	bucket, err := limiter.Bucket(pubkey)
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

func Process(_ rely.Client, request *nostr.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response := dvmHandler.Process(ctx, request)
	_, err := service.Sqlite.Save(ctx, response)
	return err
}
