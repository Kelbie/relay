package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"relay/pkg/dvm"
	"relay/pkg/eventstore"
	"relay/pkg/rate"
	"relay/pkg/req"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/fiatjaf/khatru"
	_ "github.com/joho/godotenv/autoload"
	"github.com/nbd-wtf/go-nostr"
	"github.com/redis/go-redis/v9"
	"github.com/vertex-lab/crawler/pkg/database/redisdb"
	"github.com/vertex-lab/crawler/pkg/store/redistore"
	"github.com/vertex-lab/crawler/pkg/utils/logger"
)

var env func(k string, fallback ...string) (v string)
var log *logger.Aggregate
var requestCounter = &atomic.Uint32{}
var db *eventstore.Store

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	// store secret key in .env because the bunker is too buggy at the moment
	env = Env()
	secret := env("SK")
	pubkey, err := nostr.GetPublicKey(secret)
	if err != nil {
		panic("failed to get pubkey: " + err.Error())
	}

	// initialize relay datastore, for events and white-listing.
	db, err = eventstore.New("events.sqlite")
	if err != nil {
		panic("failed to initialize database: " + err.Error())
	}
	log.Info("sqlite database connected")

	if err := RelayManagementInit(ctx, db, relay); err != nil {
		panic("failed to initialize relay management: " + err.Error())
	}

	// initialize redis connection used for computing responses
	redis := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	DB, err := redisdb.NewDatabaseConnection(ctx, redis)
	if err != nil {
		panic("redis failed to connect: " + err.Error())
	}

	RWS, err := redistore.NewRWSConnection(ctx, redis)
	if err != nil {
		panic("redis failed to connect: " + err.Error())
	}
	log.Info("redis connected")

	RateLimiter := rate.NewLimiter() // used to rate-limit requests for un-authorized users

	// setup relay
	relay.Info.Name = "Vertex Relay"
	relay.Info.Software = "Vertex Relay based on Khatru"
	relay.Info.Version = "0.0.1"
	relay.Info.PubKey = pubkey
	relay.Info.SupportedNIPs = []any{11, 42, 86, 90}

	relay.RejectEvent = append(relay.RejectEvent, RejectNonDVMs, func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
		// accept all events from authorized pubkeys
		pubkeyReasons, err := relay.ManagementAPI.ListAllowedPubKeys(ctx)
		if err != nil {
			return true, err.Error()
		}

		for _, pr := range pubkeyReasons {
			if event.PubKey == pr.PubKey {
				return false, ""
			}
		}

		// everyone else has strict rate-limits
		bucket, _ := RateLimiter.LoadOrStore(event.PubKey, rate.NewBucket())
		return bucket.Reject(1)
	})

	relay.StoreEvent = append(relay.StoreEvent, db.Save, func(ctx context.Context, event *nostr.Event) error {
		args, parsingErr := dvm.Parse(event)
		err = ProcessRequest(ctx, DB, RWS, args, parsingErr, func(ctx context.Context, res *nostr.Event) error {
			if err := res.Sign(secret); err != nil {
				return fmt.Errorf("error signing response eventID %v: %v", res.ID, err)
			}

			relay.BroadcastEvent(res)
			if err := db.Save(ctx, res); err != nil {
				return fmt.Errorf("error saving response eventID: %v, %v", res.ID, err)
			}

			return nil
		})

		if err != nil {
			log.Error("error processing event: %v", err)
			return fmt.Errorf("error processing event: %v", err)
		}
		return nil
	})
	relay.DeleteEvent = append(relay.DeleteEvent, func(ctx context.Context, event *nostr.Event) error {
		return db.Delete(ctx, event.ID)
	})
	relay.QueryEvents = append(relay.QueryEvents, QueryNoSearch, func(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
		if filter.Search == "" {
			return nil, nil
		}

		args, err := req.Parse(&filter)
		if errors.Is(err, req.ErrInvalidKindsFormat) {
			// if the filter doesn't match the valid format "kinds:<dvm_response_kind>, 7000",
			// return the error as a NOTICE and not as a kind:7000 to make sure the customer receives it.
			return nil, err
		}

		ch := make(chan *nostr.Event, 1)
		defer close(ch)

		err = ProcessRequest(ctx, DB, RWS, args, err, func(ctx context.Context, res *nostr.Event) error {
			if err := res.Sign(secret); err != nil {
				return fmt.Errorf("failed to sign eventID %v: %v", res.ID, err)
			}

			if err := db.Save(ctx, res); err != nil {
				log.Error("failed to save eventID: %v, %v", res.ID, err)
			}

			ch <- res
			return nil
		})

		if err != nil {
			log.Error("error processing event: %v", err)
			return nil, fmt.Errorf("error processing event: %w", err)
		}
		return ch, nil
	})

	go func() {
		port := env("PORT", "3334")
		log.Info("running on :%v", port)
		if err := http.ListenAndServe(fmt.Sprintf("localhost:%s", port), relay); err != nil {
			panic("failed to run relay: %" + err.Error())
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan // Block until a signal is received
	log.Info("signal received. Shutting down...")
	log.Info("total events processed: %v", requestCounter.Load())
}

// ---------------------------------HELPERS-------------------------------------

// QueryNoSearch() simply translates the slice of events returned by db.Query
// into a channel, making it compatible with Khatru.
func QueryNoSearch(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {

	if filter.Search != "" {
		return nil, nil
	}

	events, err := db.Query(ctx, &filter)
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

func RejectNonDVMs(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
	if event.Kind < 5312 || event.Kind > 5314 {
		return true, fmt.Sprintf("%v: %v", dvm.ErrInvalidKind, event.Kind)
	}
	return false, ""
}

// Env() returns a function that returns the specified enviroment variable with an optional fallback.
func Env() func(k string, fallback ...string) (v string) {
	var env = make(map[string]string)

	for _, item := range os.Environ() {
		parts := strings.SplitN(item, "=", 2)
		env[parts[0]] = parts[1]
	}

	return func(k string, fallback ...string) (v string) {
		v = env[k]

		if v == "" && len(fallback) > 0 {
			v = fallback[0]
		}

		return v
	}
}

// PrintTitle prints a simple title.
func PrintTitle(l *logger.Aggregate) {
	l.Info("----------------------")
	l.Info("Starting up the relay")
}

// PrintShutdown() prints a little shutdown message.
func PrintShutdown(l *logger.Aggregate) {
	l.Info("Relay stopped")
	l.Info("----------------------")
}
