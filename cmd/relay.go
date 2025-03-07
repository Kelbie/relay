package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/vertex-lab/relay/pkg/dvm"
	"github.com/vertex-lab/relay/pkg/eventstore"

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
	db, err = eventstore.New(env("SQLITE_URL"))
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

	// setup relay info
	relay.Info.Name = "Vertex Relay"
	relay.Info.Software = "Vertex Relay based on Khatru"
	relay.Info.Version = "0.0.1"
	relay.Info.PubKey = pubkey
	relay.Info.SupportedNIPs = []any{11, 42, 86, 90}

	relay.RejectEvent = append(relay.RejectEvent, RejectNonDVMs)

	relay.StoreEvent = append(relay.StoreEvent, db.Save, func(ctx context.Context, event *nostr.Event) error {

		err := HandleDVMRequest(ctx, DB, RWS, db, event, func(ctx context.Context, res *nostr.Event) error {
			if err := res.Sign(secret); err != nil {
				return fmt.Errorf("error signing the response eventID %s: %w", res.ID, err)
			}

			relay.BroadcastEvent(res)
			return db.Save(ctx, res)
		})

		if err != nil {
			log.Error("error processing event: %v", err)
			return fmt.Errorf("error processing event: %w", err)
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

		ch := make(chan *nostr.Event, 1)
		defer close(ch)

		err := HandleREQRequest(ctx, DB, RWS, db, &filter, func(ctx context.Context, res *nostr.Event) error {
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
		log.Info("running on :%s", port)
		if err := http.ListenAndServe(fmt.Sprintf("localhost:%s", port), relay); err != nil {
			panic("failed to run relay: %" + err.Error())
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan
	log.Info("signal received. Shutting down...")
	log.Info("total events processed: %d", requestCounter.Load())
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
	if event.Kind < 5312 || event.Kind > 5315 {
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
