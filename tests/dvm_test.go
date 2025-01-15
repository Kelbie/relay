package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"relay/pkg/dvm"
	"slices"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/crawler/pkg/crawler"
)

var (
	// pubkeys for testing
	fran  string = "726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11"
	odell string = "04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9"
	calle string = "50d94fc2d8580c682b071a542f8b1e31a200b0508bab95a33bef0855df281d63"
	pip   string = "f683e87035f7ad4f44e0b98cfbd9537e16455a92cd38cefc4cb31db7557f5ef2"

	// relay URLs
	vertexURL string = "wss://relay.vertexlab.io"
	localhost string = "http://localhost:3334"
)

func TestRelevantWhoFollow(t *testing.T) {
	// step 1. publishing a DVM request
	DVMreq := nostr.Event{
		PubKey: pip,
		Kind:   dvm.KindRelevantWhoFollow,
		Tags: nostr.Tags{
			{"param", "source", odell},
			{"param", "target", fran},
		},
	}

	sk := nostr.GeneratePrivateKey()
	if err := DVMreq.Sign(sk); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	relay, err := nostr.RelayConnect(ctx, localhost)
	if err != nil {
		t.Fatalf("failed to connect to %v: %v", vertexURL, err)
	}

	if err := relay.Publish(ctx, DVMreq); err != nil {
		t.Fatalf("failed to publish to %v: %v", vertexURL, err)
	}

	// waiting a little bit to give it time to process
	time.Sleep(500 * time.Millisecond)

	// step 2. querying for the DVM response
	filter := nostr.Filter{
		Tags: nostr.TagMap{
			"e": {DVMreq.ID},
		},
	}

	var DVMres *nostr.Event
	var counter int
	ch, err := relay.QueryEvents(ctx, filter)
	for event := range ch {
		DVMres = event
		counter++
	}

	if counter != 1 {
		t.Fatalf("expected exactly one event, got %v", counter)
	}

	// step 3. checking the response is consistent, meaning each pubkey follows target.
	if err := CheckResponseIsConsistent(DVMres, fran); err != nil {
		t.Fatal(err)
	}
}

// CheckResponseIsConsistent() parses the response's content, and checks that
// each of the pubkeys listed follows the target.
func CheckResponseIsConsistent(res *nostr.Event, target string) error {
	var ranks []dvm.RankResponse
	if err := json.Unmarshal([]byte(res.Content), &ranks); err != nil {
		return err
	}

	pubkeys := make([]string, len(ranks))
	for i, rank := range ranks {
		pubkeys[i] = rank.Pubkey
	}

	// now we query the relays for the follow-lists of the pubkeys contained in the response
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool := nostr.NewSimplePool(ctx)
	filter := nostr.Filters{
		{
			Authors: pubkeys,
			Kinds:   []int{nostr.KindFollowList},
		},
	}

	// getting only the newest follow list for each pubkey.
	newest := make(map[string]*nostr.Event, len(pubkeys))
	for event := range pool.SubManyEose(ctx, crawler.Relays, filter) {

		if _, exists := newest[event.PubKey]; !exists {
			newest[event.PubKey] = event.Event
			continue
		}

		if event.CreatedAt > newest[event.PubKey].CreatedAt {
			newest[event.PubKey] = event.Event
		}
	}

	for pubkey, event := range newest {
		follows := crawler.ParsePubkeys(event)
		if !slices.Contains(follows, fran) {
			return fmt.Errorf("%v doesn't follow %v, but the response showed otherwise:\n\n %v", pubkey, fran, res)
		}
	}

	return nil
}
