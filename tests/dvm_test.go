// these are integration tests that require a Redis instance, that can be obtained by running the crawler for about 10 minutes.
package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"relay/pkg/dvm"
	"slices"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/crawler/pkg/crawler"
)

var (
	// pubkeys for testing
	jack      string = "82341f882b6eabcd2ba7f1ef90aad961cf074af15b9ef44a09f9d2a8fbfbe6a2"
	damus     string = "3efdaebb1d8923ebd99c9e7ace3b4194ab45512e2be79c1b7d68d9243e0d2681"
	jb55      string = "32e1827635450ebb3c5a7d12c1f8e7b2b514439ac10a67eef3d9fd9c5c68e245"
	snowden   string = "84dee6e676e5bb67b4ad4e042cf70cbd8681155db535942fcc6a0533858a7240"
	fran      string = "726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11"
	odell     string = "04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9"
	calle     string = "50d94fc2d8580c682b071a542f8b1e31a200b0508bab95a33bef0855df281d63"
	pip       string = "f683e87035f7ad4f44e0b98cfbd9537e16455a92cd38cefc4cb31db7557f5ef2"
	randomKey string = "d5ad3d3115d9fa07500b06ccd0b9605d9888a206acba20a1e2e681ec29109387"

	// relay URLs
	vertexURL string = "wss://relay.vertexlab.io"
	localhost string = "http://localhost:3334"
)

func TestDVM_VerifyReputation(t *testing.T) {
	// step 1. publishing a DVM request
	DVMreq := nostr.Event{
		Kind: dvm.KindVerifyReputation,
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
		t.Fatalf("failed to connect to %v: %v", localhost, err)
	}

	if err := relay.Publish(ctx, DVMreq); err != nil {
		t.Fatalf("failed to publish to %v: %v", localhost, err)
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

	if DVMres.Kind != dvm.KindVerifyReputation+1000 {
		t.Errorf("expected DVM response to have kind %d, got %d", dvm.KindVerifyReputation+1000, DVMres.Kind)
		t.Fatalf("DVM res: %v", DVMres)
	}

	var ranks []dvm.RankResponse
	if err := json.Unmarshal([]byte(DVMres.Content), &ranks); err != nil {
		t.Errorf("failed to unmarshal the DVM response content: %v", err)
	}

	if ranks[0].Pubkey != fran {
		t.Errorf("the first pubkey should be the target %s", fran)
	}

	// step 3. checking each pubkey follows target.
	if err := checkFollowers(ranks[1:], fran); err != nil {
		t.Fatal(err)
	}
}

func TestDVM_SortAuthors(t *testing.T) {
	// step 1. publishing a DVM request
	DVMreq := nostr.Event{
		Kind: dvm.KindSortAuthors,
		Tags: nostr.Tags{
			{"param", "source", odell},
			{"param", "target", calle},
			{"param", "target", fran},
			{"param", "target", randomKey},
		},
	}

	expectedSortedPubkeys := []string{calle, fran, randomKey}

	sk := nostr.GeneratePrivateKey()
	if err := DVMreq.Sign(sk); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	relay, err := nostr.RelayConnect(ctx, localhost)
	if err != nil {
		t.Fatalf("failed to connect to %v: %v", localhost, err)
	}

	if err := relay.Publish(ctx, DVMreq); err != nil {
		t.Fatalf("failed to publish to %v: %v", localhost, err)
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

	if DVMres.Kind != dvm.KindSortAuthors+1000 {
		t.Errorf("expected DVM response to have kind %d, got %d", dvm.KindSortAuthors+1000, DVMres.Kind)
		t.Fatalf("DVM res: %v", DVMres)
	}

	var ranks []dvm.RankResponse
	if err := json.Unmarshal([]byte(DVMres.Content), &ranks); err != nil {
		t.Errorf("failed to unmarshal the DVM response content: %v", err)
	}

	pubkeys := make([]string, len(ranks))
	for i, rank := range ranks {
		pubkeys[i] = rank.Pubkey
	}

	if !reflect.DeepEqual(pubkeys, expectedSortedPubkeys) {
		t.Fatalf("expected sorted pubkeys %v, got %v", expectedSortedPubkeys, pubkeys)
	}
}

func TestDVM_RecommendFollows(t *testing.T) {
	// step 1. publishing a DVM request
	DVMreq := nostr.Event{
		Kind: dvm.KindRecommendFollows,
		Tags: nostr.Tags{
			{"param", "source", randomKey},
		},
	}

	// this is list is dependent on the database
	expectedRecommendations := []string{damus, jack, jb55, snowden, odell}

	sk := nostr.GeneratePrivateKey()
	if err := DVMreq.Sign(sk); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	relay, err := nostr.RelayConnect(ctx, localhost)
	if err != nil {
		t.Fatalf("failed to connect to %v: %v", localhost, err)
	}

	if err := relay.Publish(ctx, DVMreq); err != nil {
		t.Fatalf("failed to publish to %v: %v", localhost, err)
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

	if DVMres.Kind != dvm.KindRecommendFollows+1000 {
		t.Errorf("expected DVM response to have kind %d, got %d", dvm.KindRecommendFollows+1000, DVMres.Kind)
		t.Fatalf("DVM res: %v", DVMres)
	}

	var ranks []dvm.RankResponse
	if err := json.Unmarshal([]byte(DVMres.Content), &ranks); err != nil {
		t.Errorf("failed to unmarshal the DVM response content: %v", err)
	}

	recommendations := make([]string, len(ranks))
	for i, rank := range ranks {
		recommendations[i] = rank.Pubkey
	}

	if !reflect.DeepEqual(recommendations, expectedRecommendations) {
		t.Errorf("recommendations don't match the expected ones")
		t.Errorf("recommendations:")
		for i, pk := range recommendations {
			t.Errorf("%d) %s", i, pk)
		}
		t.Errorf("expected:")
		for i, pk := range expectedRecommendations {
			t.Errorf("%d) %s", i, pk)
		}
	}

}

// checkFollowers() and checks that each of the pubkeys listed in the response follows the target.
func checkFollowers(ranks []dvm.RankResponse, target string) error {
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

	if len(newest) != len(pubkeys) {
		return fmt.Errorf("expected to receive one follow-list per pubkey (%d), got %d", len(pubkeys), len(newest))
	}

	for pubkey, event := range newest {
		follows := crawler.ParsePubkeys(event)
		if !slices.Contains(follows, target) {
			return fmt.Errorf("%v doesn't follow %v, but the response showed otherwise:\n\n %v", pubkey, fran, ranks)
		}
	}

	return nil
}
