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

var reqChan = make(chan *nostr.Event, 1000)
var env func(k string, fallback ...string) (v string)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger, logFile := logger.Init("relay.log")
	defer logFile.Close()

	PrintTitle(logger)
	go HandleSignals(cancel, logger)

	env = Env()
	relay := khatru.NewRelay()

	db := &sqlite3.SQLite3Backend{DatabaseURL: "relay.sqlite"}
	if err := db.Init(); err != nil {
		logger.Error("database failed to initialize: %v", err)
		fmt.Printf("\ndatabase failed to initialize: %v", err)
		panic(err)
	}
	fmt.Println("database connected")

	bunker, err := nip46.ConnectBunker(ctx, nostr.GeneratePrivateKey(), env("BUNKER"), nil, nil)
	if err != nil {
		logger.Error("bunker failed to initialize with %v", env("BUNKER"))
		fmt.Println("bunker failed to initialize with", env("BUNKER"))
		panic(err)
	}
	fmt.Println("bunker connected with", env("BUNKER"))

	if err := RelayManagementInit(ctx, db, relay); err != nil {
		logger.Error("relay management failed to initialize: %v", err)
		fmt.Printf("\nrelay management failed to initialize: %v", err)
		panic(err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	relay.Info.Name = "Vertex Relay"
	relay.Info.Software = "Vertex Relay based on Khatru"
	relay.Info.Version = "0.0.1"
	relay.Info.SupportedNIPs = append(relay.Info.SupportedNIPs, []int{90}...)
	relay.Info.PubKey, _ = bunker.GetPublicKey(ctx)

	relay.QueryEvents = append(relay.QueryEvents, db.QueryEvents)
	relay.DeleteEvent = append(relay.DeleteEvent, db.DeleteEvent)
	relay.RejectEvent = append(relay.RejectEvent, func(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
		if event.Kind < 5312 || event.Kind > 5318 {
			return true, "invalid kind"
		}
		return false, ""
	})

	// before storing the event, send it to the request queue to be processed
	relay.StoreEvent = append(relay.StoreEvent, func(ctx context.Context, event *nostr.Event) error {
		reqChan <- event
		return db.SaveEvent(ctx, event)
	})
	ProcessRequests(ctx, logger, client, bunker, relay, reqChan)

	mux := relay.Router()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("content-type", "text/html")
		fmt.Fprintf(w, "Welcome to Vertex Relay")
	})

	// Launch server in a goroutine to allow initializing bunker afterwards (depends on this relay)
	go func() {
		port := env("PORT", "3334")
		http.ListenAndServe(fmt.Sprintf("localhost:%s", port), relay)
		fmt.Printf("running on :%s\n", port)
	}()
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
	fmt.Println("----------------------")
	fmt.Println("Starting up the relay")
	fmt.Println("----------------------")

	l.Info("----------------------")
	l.Info("Starting up the relay")
	l.Info("----------------------")
}
