package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/fiatjaf/eventstore/sqlite3"
	"github.com/fiatjaf/khatru"
	"github.com/go-redis/redis"
	_ "github.com/joho/godotenv/autoload"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip46"
)

var env func(k string, fallback ...string) (v string)
var BunkerClient *nip46.BunkerClient
var RedisClient *redis.Client

func main() {
	env = getEnv()

	relay := khatru.NewRelay()

	db := sqlite3.SQLite3Backend{DatabaseURL: "relay.sqlite"}
	if err := db.Init(); err != nil {
		panic(err)
	}

	relay.StoreEvent = append(relay.StoreEvent, db.SaveEvent, func(ctx context.Context, event *nostr.Event) error {
		if event.Kind >= 5312 && event.Kind <= 5316 {
			resultChannel := make(chan nostr.Event)
			switch event.Kind {
			case 5312:
				go RelevantWhoFollowHandler(event, resultChannel)
			case 5313:
				go RecommendedFollowsHandler(event, resultChannel)
			case 5314:
				go SortAuthorsHandler(event, resultChannel)
			case 5315:
				go ImpersonatorDetectionHandler(event, resultChannel)
			case 5316:
				go DegreesOfSeparationHandler(event, resultChannel)
			}
			result := <-resultChannel
			// TODO: Do not save for now
			// db.SaveEvent(ctx, &result)
			relay.BroadcastEvent(&result)
		}
		return nil
	})

	relay.QueryEvents = append(relay.QueryEvents, db.QueryEvents)
	relay.DeleteEvent = append(relay.DeleteEvent, db.DeleteEvent)

	mux := relay.Router()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("content-type", "text/html")
		fmt.Fprintf(w, "Welcome to Vertex Relay")
	})

	port := env("PORT", "3334")

	fmt.Printf("running on :%s\n", port)
	http.ListenAndServe(fmt.Sprintf("localhost:%s", port), relay)

	// Initialize clients
	RedisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	fmt.Println("Attempting to connect to bunker at", env("BUNKER"))
	bunker, err := nip46.ConnectBunker(context.Background(), RelayPrivateKey, env("BUNKER"), nil, nil)

	if err != nil {
		panic(err)
	}
	fmt.Println("Bunker connected")
	BunkerClient = bunker

	relay.Info.Name = "Vertex Relay"
	relay.Info.Software = "Vertex Relay based on Khatru"
	relay.Info.Version = "0.0.1"
	relay.Info.SupportedNIPs = append(relay.Info.SupportedNIPs, []int{90}...)
	relay.Info.PubKey, _ = bunker.GetPublicKey(context.Background())
}
