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
	req := &nostr.Event{
		Kind: dvm.KindVerifyReputation,
		Tags: nostr.Tags{
			{"param", "source", odell},
			{"param", "target", fran},
		},
	}

	sk := nostr.GeneratePrivateKey()
	pk, err := nostr.GetPublicKey(sk)
	if err != nil {
		t.Fatalf("failed to get pk from sk: %v", err)
	}

	if err := req.Sign(sk); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	expectedTags := nostr.Tags{{"e", req.ID}, {"p", pk}}
	expectedKind := req.Kind + 1000

	res, err := dvmResponse(req, localhost)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkFormat(res, expectedKind, expectedTags); err != nil {
		t.Errorf("the format of the response is wrong: %v", err)
	}

	var ranks dvm.RankResponses
	if err := json.Unmarshal([]byte(res.Content), &ranks); err != nil {
		t.Fatalf("failed to unmarshal the DVM response content: %v", err)
	}

	pubkeys, _ := ranks.Unpack()
	if pubkeys[0] != fran {
		t.Errorf("the first pubkey should be the target %s", fran)
	}

	if err := checkPubkeysFollowTarget(pubkeys[1:], fran); err != nil {
		t.Fatal(err)
	}
}

func TestDVM_SortAuthors(t *testing.T) {
	// step 1. publishing a DVM request
	req := &nostr.Event{
		Kind: dvm.KindSortAuthors,
		Tags: nostr.Tags{
			{"param", "source", odell},
			{"param", "target", calle},
			{"param", "target", fran},
			{"param", "target", randomKey},
		},
	}

	sk := nostr.GeneratePrivateKey()
	pk, err := nostr.GetPublicKey(sk)
	if err != nil {
		t.Fatalf("failed to get pk from sk: %v", err)
	}

	if err := req.Sign(sk); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	expectedSorted := []string{calle, fran, randomKey}
	expectedTags := nostr.Tags{{"e", req.ID}, {"p", pk}}
	expectedKind := req.Kind + 1000

	res, err := dvmResponse(req, localhost)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkFormat(res, expectedKind, expectedTags); err != nil {
		t.Errorf("the format of the response is wrong: %v", err)
	}

	var ranks dvm.RankResponses
	if err := json.Unmarshal([]byte(res.Content), &ranks); err != nil {
		t.Errorf("failed to unmarshal the DVM response content: %v", err)
	}

	sorted, _ := ranks.Unpack()
	if !reflect.DeepEqual(sorted, expectedSorted) {
		t.Errorf("sorted keys don't match the expected ones")
		t.Errorf("sorted:")
		for i, pk := range sorted {
			t.Errorf("%d) %s", i, pk)
		}
		t.Errorf("expected:")
		for i, pk := range expectedSorted {
			t.Errorf("%d) %s", i, pk)
		}
	}
}

func TestDVM_RecommendFollows(t *testing.T) {
	req := &nostr.Event{
		Kind: dvm.KindRecommendFollows,
		Tags: nostr.Tags{
			{"param", "source", randomKey},
		},
	}

	sk := nostr.GeneratePrivateKey()
	pk, err := nostr.GetPublicKey(sk)
	if err != nil {
		t.Fatalf("failed to get pk from sk: %v", err)
	}

	if err := req.Sign(sk); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	// this list is dependent on the specific database
	expectedRecommendations := []string{damus, jack, jb55, snowden, odell}
	expectedTags := nostr.Tags{{"e", req.ID}, {"p", pk}}
	expectedKind := req.Kind + 1000

	res, err := dvmResponse(req, localhost)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkFormat(res, expectedKind, expectedTags); err != nil {
		t.Errorf("the format of the response is wrong: %v", err)
	}

	var ranks dvm.RankResponses
	if err := json.Unmarshal([]byte(res.Content), &ranks); err != nil {
		t.Errorf("failed to unmarshal the DVM response content: %v", err)
	}

	recommendations, _ := ranks.Unpack()
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

// -----------------------------------HELPERS----------------------------------

// dvmResponse() connects to the relay, send the request and fetches the response using the request ID.
func dvmResponse(req *nostr.Event, relayURL string) (res *nostr.Event, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	relay, err := nostr.RelayConnect(ctx, relayURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", relayURL, err)
	}

	if err := relay.Publish(ctx, *req); err != nil {
		return nil, fmt.Errorf("failed to publish to %s: %v", relayURL, err)
	}

	filter := nostr.Filter{
		Tags: nostr.TagMap{
			"e": {req.ID},
		},
	}

	var counter int
	ch, err := relay.QueryEvents(ctx, filter)
	for event := range ch {
		res = event
		counter++
	}

	if counter != 1 {
		return nil, fmt.Errorf("expected exactly one response, got %v", counter)
	}

	return res, nil
}

// checkPubkeysFollowTarget() checks that each of the pubkeys listed in the response follows the target.
func checkPubkeysFollowTarget(pubkeys []string, target string) error {
	for _, pk := range pubkeys {
		if !nostr.IsValidPublicKey(pk) {
			return fmt.Errorf("%s is not a valid public key", pk)
		}
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
			return fmt.Errorf("%v doesn't follow %v, but the response showed otherwise", pubkey, target)
		}
	}

	return nil
}

// checkFormat() checks that the "format" (tags) of the event matches what it should be.
func checkFormat(event *nostr.Event, kind int, tags nostr.Tags) error {
	if event == nil {
		return fmt.Errorf("nil event")
	}

	if event.Kind != kind {
		return fmt.Errorf("expected kind %d, got %d", kind, event.Kind)
	}

	for _, t := range tags {
		if !contains(event.Tags, t) {
			return fmt.Errorf("the tag %v is not present in the event's tags", t)
		}
	}

	return nil
}

// contains() returns whether tags contains a specified tag
func contains(tags nostr.Tags, tag nostr.Tag) bool {
	for _, t := range tags {
		if reflect.DeepEqual(tag, t) {
			return true
		}
	}

	return false
}
