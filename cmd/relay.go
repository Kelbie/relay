package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fiatjaf/eventstore/sqlite3"
	"github.com/fiatjaf/khatru"
	_ "github.com/joho/godotenv/autoload"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip46"
	"github.com/redis/go-redis/v9"
	"github.com/vertex-lab/crawler/pkg/utils/logger"
)

// TODO; wrap event/filter in a structure that contains the context of such request as well.
var DVMQueue = make(chan *nostr.Event, 1000)     // queue where the DVM events are processed.
var filterQueue = make(chan *nostr.Filter, 1000) // queue where the REQ filters are processed.

var env func(k string, fallback ...string) (v string)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	relay := khatru.NewRelay()
	env = Env()

	logger := logger.New(os.Stdout)
	relay.Log.SetOutput(os.Stdout)

	PrintTitle(logger)
	defer PrintShutdown(logger)

	go HandleSignals(cancel, logger)

	// initialize relay datastore, for events and whitelisting.
	db := &sqlite3.SQLite3Backend{DatabaseURL: "relay.sqlite"}
	if err := db.Init(); err != nil {
		panic(err)
	}
	logger.Info("database connected")

	if err := RelayManagementInit(ctx, db, relay); err != nil {
		panic(err)
	}

	mux := relay.Router()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("content-type", "text/html")
		fmt.Fprintf(w, "Welcome to Vertex Relay")
	})

	// Launch server in a goroutine to allow initializing bunker afterwards (depends on this relay)
	go func() {
		port := env("PORT", "3334")
		logger.Info("running on :%s\n", port)
		if err := http.ListenAndServe(fmt.Sprintf("localhost:%s", port), relay); err != nil {
			logger.Error("failed to listen %v", err)
		}
	}()

	// initialize the bunker used for signing events
	bunker, err := nip46.ConnectBunker(ctx, nostr.GeneratePrivateKey(), env("BUNKER"), nil, nil)
	if err != nil {
		panic(err)
	}
	pubkey, err := bunker.GetPublicKey(ctx)
	if err != nil {
		panic(err)
	}
	logger.Info("\nbunker connected with url %v, pubkey %v", env("BUNKER"), pubkey)

	// initialize redis connection, used for computing responses
	redis := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

	// setup relay
	relay.Info.Name = "Vertex Relay"
	relay.Info.Software = "Vertex Relay based on Khatru"
	relay.Info.Version = "0.0.1"
	relay.Info.PubKey = pubkey
	relay.Info.SupportedNIPs = append(relay.Info.SupportedNIPs, 90)

	// event pipeline
	relay.RejectEvent = append(relay.RejectEvent, func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
		if event.Kind < 5312 || event.Kind > 5318 {
			return true, "invalid kind"
		}

		DVMQueue <- event // send to the DVM queue if valid
		return false, ""
	})
	relay.DeleteEvent = append(relay.DeleteEvent, db.DeleteEvent)
	relay.StoreEvent = append(relay.StoreEvent, db.SaveEvent)

	// request pipeline
	relay.RejectFilter = append(relay.RejectFilter, func(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
		if len(filter.Kinds) != 1 {
			return true, "request exactly one kind at the time"
		}

		if filter.Kinds[0] < 6312 || filter.Kinds[0] > 6318 {
			return true, "invalid kind"
		}

		return false, ""
	})
	relay.QueryEvents = append(relay.QueryEvents, func(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
		ch, err := db.QueryEvents(ctx, filter)
		if err != nil {
			return nil, err
		}

		// If there are matches, send them.
		if len(ch) != 0 {
			return ch, nil
		}

		// if there are no matches (e.g. the stored event is older than the specified "Since"),
		// then send the filter to the queue to produce a new event.
		filterQueue <- &filter
		return nil, nil
	})

	ProcessRequests(ctx, logger, redis, DVMQueue, filterQueue, func(ctx context.Context, res *nostr.Event) error {
		if err := bunker.SignEvent(ctx, res); err != nil {
			return err
		}

		relay.BroadcastEvent(res)
		return nil
	})
}

// ---------------------------------HELPERS------------------------------------

// HandleSignals() listens for OS signals and triggers context cancellation.
func HandleSignals(cancel context.CancelFunc, logger *logger.Aggregate) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	<-signalChan // Block until a signal is received
	fmt.Printf("\nSignal received. Shutting down...")
	logger.Info("Signal received. Shutting down...")
	cancel()
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
