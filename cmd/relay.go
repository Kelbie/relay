package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"

	"github.com/vertex-lab/relay/pkg/dvm"
	"github.com/vertex-lab/relay/pkg/eventstore"
	"github.com/vertex-lab/relay/pkg/rate"
	"github.com/vertex-lab/relay/pkg/req"

	"github.com/fiatjaf/khatru"
	_ "github.com/joho/godotenv/autoload"
	"github.com/nbd-wtf/go-nostr"
	"github.com/redis/go-redis/v9"
	"github.com/vertex-lab/crawler/pkg/database/redisdb"
	"github.com/vertex-lab/crawler/pkg/store/redistore"
	"github.com/vertex-lab/crawler/pkg/utils/logger"
)

var config *Config
var err error
var log *logger.Aggregate
var requestCounter = &atomic.Uint32{}
var sqlite *eventstore.Store
var limiter rate.Limiter

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config, err = LoadConfig()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	relay := khatru.NewRelay()
	mux := relay.Router()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("content-type", "text/html")
		fmt.Fprintf(w, "Welcome to Vertex Relay")
	})

	log = logger.New(os.Stdout)
	relay.Log.SetOutput(os.Stdout)
	nostr.DebugLogger.SetOutput(os.Stdout)
	nostr.InfoLogger.SetOutput(io.Discard) // discarding info logs

	PrintTitle(log)
	defer PrintShutdown(log)

	sqlite, err = eventstore.New(config.SQLitePath)
	if err != nil {
		panic("failed to initialize sqlite with path " + config.SQLitePath + ": " + err.Error())
	}
	log.Info("sqlite connected to %s", config.SQLitePath)

	redis := redis.NewClient(&redis.Options{Addr: config.RedisAddress})
	limiter = rate.NewLimiterWithPolicy(redis, config.Limits)
	DB, err := redisdb.NewDatabaseConnection(ctx, redis)
	if err != nil {
		panic("failed to connect to redis on " + config.RedisAddress + ": " + err.Error())
	}

	RWS, err := redistore.NewRWSConnection(ctx, redis)
	if err != nil {
		panic("failed to connect to redis on " + config.RedisAddress + ": " + err.Error())
	}
	log.Info("redis connected at %s", config.RedisAddress)

	relay.Info.Name = "Vertex Relay"
	relay.Info.Software = "Vertex Relay based on Khatru"
	relay.Info.Version = "0.0.1"
	relay.Info.PubKey = config.public
	relay.Info.SupportedNIPs = []any{11, 42, 86, 90}

	relay.RejectEvent = append(relay.RejectEvent, RejectNonDVMs)

	relay.StoreEvent = append(relay.StoreEvent, sqlite.Save, func(ctx context.Context, event *nostr.Event) error {
		err := HandleDVMRequest(ctx, DB, RWS, sqlite, event, func(ctx context.Context, res *nostr.Event) error {
			if err := res.Sign(config.secret); err != nil {
				return fmt.Errorf("error signing the response eventID %s: %w", res.ID, err)
			}

			relay.BroadcastEvent(res)
			return sqlite.Save(ctx, res)
		})

		if err != nil {
			log.Error("error processing event: %v", err)
			return fmt.Errorf("error processing event: %w", err)
		}

		return nil
	})

	relay.DeleteEvent = append(relay.DeleteEvent, func(ctx context.Context, event *nostr.Event) error {
		return sqlite.Delete(ctx, event.ID)
	})

	relay.QueryEvents = append(relay.QueryEvents, AuthedInfo, QueryNoSearch, func(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
		if filter.Search == "" {
			return nil, nil
		}

		ch := make(chan *nostr.Event, 1)
		defer close(ch)

		err := HandleREQRequest(ctx, DB, RWS, sqlite, &filter, func(ctx context.Context, res *nostr.Event) error {
			if err := res.Sign(config.secret); err != nil {
				return fmt.Errorf("failed to sign eventID %v: %v", res.ID, err)
			}

			if err := sqlite.Save(ctx, res); err != nil {
				log.Error("failed to save eventID: %v, %v", res.ID, err)
			}

			ch <- res
			return nil
		})

		if err != nil && !errors.Is(err, req.ErrInvalidKindsFormat) {
			log.Error("error processing event: %v", err)
			return nil, fmt.Errorf("error processing event: %w", err)
		}

		return ch, nil
	})

	go func() {
		if err := http.ListenAndServe(config.RelayAddress, relay); err != nil {
			panic("failed to run relay on " + config.RelayAddress + ": " + err.Error())
		}
	}()
	log.Info("relay running on %s", config.RelayAddress)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan
	log.Info("signal received. Shutting down...")
	log.Info("total events processed: %d", requestCounter.Load())
}

// ---------------------------------HELPERS-------------------------------------

func AuthedInfo(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
	if len(filter.Kinds) != 1 || filter.Kinds[0] != 22243 {
		return nil, nil
	}

	pubkey := khatru.GetAuthed(ctx)
	if pubkey == "" {
		khatru.RequestAuth(ctx)
		return nil, fmt.Errorf("auth-required: you must be authenticated to request your credit balance")
	}

	bucket, err := limiter.Bucket(pubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bucket: %w", err)
	}

	info := nostr.Event{
		Kind:      22243,
		CreatedAt: nostr.Now(),
		Tags: nostr.Tags{
			{"credits", strconv.Itoa(bucket.Tokens)},
			{"lastRequest", strconv.FormatInt(bucket.LastModified, 10)},
		},
	}

	if err := info.Sign(config.secret); err != nil {
		return nil, fmt.Errorf("failed to sign auth info: %w", err)
	}

	ch := make(chan *nostr.Event, 1)
	defer close(ch)

	ch <- &info
	return ch, nil
}

// QueryNoSearch() simply translates the slice of events returned by sqlite.Query
// into a channel, making it compatible with Khatru.
func QueryNoSearch(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
	if filter.Search != "" {
		return nil, nil
	}

	events, err := sqlite.Query(ctx, &filter)
	if err != nil {
		return nil, err
	}

	ch := make(chan *nostr.Event, len(events))
	defer close(ch)

	for _, e := range events {
		ch <- &e
	}

	return ch, nil
}

func RejectNonDVMs(ctx context.Context, event *nostr.Event) (bool, string) {
	if event.Kind < 5312 || event.Kind > 5315 {
		return true, fmt.Sprintf("%v: %v", dvm.ErrInvalidKind, event.Kind)
	}
	return false, ""
}

func PrintTitle(l *logger.Aggregate) {
	l.Info("----------------------")
	l.Info("Starting up the relay")
}

func PrintShutdown(l *logger.Aggregate) {
	l.Info("Relay stopped")
	l.Info("----------------------")
}
