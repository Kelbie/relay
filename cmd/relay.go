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

	// store secret key in .env because the bunker is too buggy at the moment
	secret := env("SK")
	pubkey, err := nostr.GetPublicKey(secret)
	if err != nil {
		panic("failed to get pubkey:" + err.Error())
	}

	logger := logger.New(os.Stdout)
	relay.Log.SetOutput(os.Stdout)

	PrintTitle(logger)
	defer PrintShutdown(logger)

	go HandleSignals(cancel, logger)

	// initialize relay datastore, for events and whitelisting.
	db := &sqlite3.SQLite3Backend{DatabaseURL: "relay.sqlite"}
	if err := db.Init(); err != nil {
		panic("failed to initialize database" + err.Error())
	}
	logger.Info("sqlite database connected")

	if err := RelayManagementInit(ctx, db, relay); err != nil {
		panic("failed to initialize relay management" + err.Error())
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
		logger.Info("running on :%v", port)
		if err := http.ListenAndServe(fmt.Sprintf("localhost:%s", port), relay); err != nil {
			panic("failed to run relay: %" + err.Error())
		}
	}()

	// initialize redis connection used for computing responses
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

		DVMQueue <- event // send to the DVM queue for processing
		return false, ""
	})
	relay.DeleteEvent = append(relay.DeleteEvent, db.DeleteEvent)
	relay.StoreEvent = append(relay.StoreEvent, db.SaveEvent)

	// request pipeline
	relay.RejectFilter = append(relay.RejectFilter, func(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
		if filter.Search != "" {
			filterQueue <- &filter // send to the filter queue for processing
		}
		return false, ""
	})
	relay.QueryEvents = append(relay.QueryEvents, db.QueryEvents)

	ProcessRequests(ctx, logger, redis, DVMQueue, filterQueue, func(ctx context.Context, res *nostr.Event) error {
		if err := res.Sign(secret); err != nil {
			logger.Error("error signing response eventID %v: %v", res.ID, err)
		}

		relay.BroadcastEvent(res)
		if err := db.SaveEvent(ctx, res); err != nil {
			logger.Error("error saving response eventID %v: %v", res.ID, err)
		}
		return nil
	})
}

// ---------------------------------HELPERS------------------------------------

// HandleSignals() listens for OS signals and triggers context cancellation.
func HandleSignals(cancel context.CancelFunc, logger *logger.Aggregate) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	<-signalChan // Block until a signal is received
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

// -----------------------------------SCRAP------------------------------------

// bunkerCtx, bunkerCancel := context.WithTimeout(ctx, 10*time.Second)
// defer bunkerCancel()

// // initialize the bunker used for signing events
// bunker, err := nip46.ConnectBunker(bunkerCtx, nostr.GeneratePrivateKey(), env("BUNKER"), nil, nil)
// if err != nil {
// 	panic(fmt.Errorf("failed to connect to bunker: %v", err))
// }
// pubkey, err := bunker.GetPublicKey(ctx)
// if err != nil {
// 	panic(fmt.Errorf("failed to get public: %v", err))
// }
// logger.Info("bunker connected with pubkey %v", pubkey)
