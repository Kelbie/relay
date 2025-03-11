// these are integration tests that require a Redis instance, that can be obtained by running the crawler for about 10 minutes.
package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vertex-lab/relay/pkg/dvm"
	"github.com/vertex-lab/relay/pkg/rate"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/crawler/pkg/crawler"
)

var (
	// pubkeys for testing
	jack_dorsey  string = "82341f882b6eabcd2ba7f1ef90aad961cf074af15b9ef44a09f9d2a8fbfbe6a2"
	jack_mallers string = "c4eabae1be3cf657bc1855ee05e69de9f059cb7a059227168b80b89761cbc4e0"
	jack_spirko  string = "a1fc5dfd7ffcf563c89155b466751b580d115e136e2f8c90e8913385bbedb1cf"
	damus        string = "3efdaebb1d8923ebd99c9e7ace3b4194ab45512e2be79c1b7d68d9243e0d2681"
	jb55         string = "32e1827635450ebb3c5a7d12c1f8e7b2b514439ac10a67eef3d9fd9c5c68e245"
	snowden      string = "84dee6e676e5bb67b4ad4e042cf70cbd8681155db535942fcc6a0533858a7240"
	fran         string = "726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11"
	odell        string = "04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9"
	calle        string = "50d94fc2d8580c682b071a542f8b1e31a200b0508bab95a33bef0855df281d63"
	pip          string = "f683e87035f7ad4f44e0b98cfbd9537e16455a92cd38cefc4cb31db7557f5ef2"
	randomKey    string = "d5ad3d3115d9fa07500b06ccd0b9605d9888a206acba20a1e2e681ec29109387"

	// relay URLs
	vertexURL     string = "wss://relay.vertexlab.io"
	localhost     string = "http://localhost:3334"
	defaultRelays        = []string{"wss://relay.primal.net", "wss://relay.nostr.band", "wss://relay.damus.io"}

	// secret not so secret key
	sk = "140494e2df64262cf14849db6e6e5333bca3e1d465cdd1acf2329a20a11b0b9c"
	pk = "3b0bc9a352e3b471b39879892c2116c1dc70f2aaff374f3cd01473ce19b2dcb4"
)

// add tokens to the bucket of `pk` so the subsequent tests will not be rejected
func TestInit(t *testing.T) {
	redis := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	limiter := rate.NewLimiter(redis)

	if _, err := limiter.TopUp(pk, 100); err != nil {
		t.Fatalf("failed to add credits to test pubkey: %v", err)
	}
}

func TestRateLimiting(t *testing.T) {
	req := &nostr.Event{
		Kind: dvm.KindVerifyReputation,
		Tags: nostr.Tags{
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

	expectedKind := dvm.KindDVMError
	expectedTags := nostr.Tags{{"e", req.ID}, {"p", pk}, {"status", "error", dvm.ErrNoCredits.Error()}}

	res, err := dvmResponse(req, localhost)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkFormat(res, expectedKind, expectedTags); err != nil {
		t.Fatalf("the format of the response is wrong: %v", err)
	}
}

func TestDVM_VerifyReputation(t *testing.T) {
	req := &nostr.Event{
		Kind:      dvm.KindVerifyReputation,
		CreatedAt: nostr.Now(),
		Tags: nostr.Tags{
			{"param", "source", odell},
			{"param", "target", fran},
		},
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
		t.Fatalf("the format of the response is wrong: %v", err)
	}

	var ranks dvm.PubkeyRanks
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

func TestDVM_SortProfiles(t *testing.T) {
	// step 1. publishing a DVM request
	req := &nostr.Event{
		Kind:      dvm.KindSortProfiles,
		CreatedAt: nostr.Now(),
		Tags: nostr.Tags{
			{"param", "source", odell},
			{"param", "target", calle},
			{"param", "target", fran},
			{"param", "target", randomKey},
		},
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
		t.Fatalf("the format of the response is wrong: %v", err)
	}

	var ranks dvm.PubkeyRanks
	if err := json.Unmarshal([]byte(res.Content), &ranks); err != nil {
		t.Fatalf("failed to unmarshal the DVM response content: %v", err)
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
		Kind:      dvm.KindRecommendFollows,
		CreatedAt: nostr.Now(),
		Tags: nostr.Tags{
			{"param", "source", randomKey},
			{"param", "limit", "3"},
		},
	}

	if err := req.Sign(sk); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	// this list is dependent on the specific database
	expectedRecommendations := []string{damus, jack_dorsey, jb55}
	expectedTags := nostr.Tags{{"e", req.ID}, {"p", pk}}
	expectedKind := req.Kind + 1000

	res, err := dvmResponse(req, localhost)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkFormat(res, expectedKind, expectedTags); err != nil {
		t.Fatalf("the format of the response is wrong: %v", err)
	}

	var ranks dvm.PubkeyRanks
	if err := json.Unmarshal([]byte(res.Content), &ranks); err != nil {
		t.Fatalf("failed to unmarshal the DVM response content: %v", err)
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

func TestDVM_SearchProfiles(t *testing.T) {
	// step 1. publishing a DVM request
	req := &nostr.Event{
		Kind:      dvm.KindSearchProfiles,
		CreatedAt: nostr.Now(),
		Tags: nostr.Tags{
			{"param", "search", "jack"},
			{"param", "limit", "3"},
		},
	}

	if err := req.Sign(sk); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	expectedJacks := []string{jack_dorsey, jack_mallers, jack_spirko}
	expectedTags := nostr.Tags{{"e", req.ID}, {"p", pk}}
	expectedKind := req.Kind + 1000

	res, err := dvmResponse(req, localhost)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkFormat(res, expectedKind, expectedTags); err != nil {
		t.Fatalf("the format of the response is wrong: %v", err)
	}

	var ranks dvm.PubkeyRanks
	if err := json.Unmarshal([]byte(res.Content), &ranks); err != nil {
		t.Fatalf("failed to unmarshal the DVM response content: %v", err)
	}

	jacks, _ := ranks.Unpack()
	if !reflect.DeepEqual(jacks, expectedJacks) {
		t.Errorf("the search results don't match the expected ones")
		t.Errorf("result:")
		for i, pk := range jacks {
			t.Errorf("%d) %s", i, pk)
		}
		t.Errorf("expected:")
		for i, pk := range expectedJacks {
			t.Errorf("%d) %s", i, pk)
		}
	}
}

// -----------------------------------HELPERS----------------------------------

// dvmResponse() connects to the relay, send the request and fetches the response using the request ID.
func dvmResponse(req *nostr.Event, relayURL string) (res *nostr.Event, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	relay, err := nostr.RelayConnect(ctx, relayURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", relayURL, err)
	}

	if err := relay.Publish(ctx, *req); err != nil {
		return nil, fmt.Errorf("failed to publish to %s: %v", relayURL, err)
	}

	time.Sleep(1 * time.Second)

	filter := nostr.Filter{
		Kinds: []int{req.Kind + 1000, dvm.KindDVMError},
		Tags: nostr.TagMap{
			"e": {req.ID},
		},
	}

	ch, err := relay.QueryEvents(ctx, filter)
	if err != nil {
		return nil, err
	}

	var counter int
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
	for event := range pool.SubManyEose(ctx, defaultRelays, filter) {

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
	if event.Kind != kind {
		return fmt.Errorf("expected kind %d, got %d:\n event %v", kind, event.Kind, event)
	}

	for _, t := range tags {
		if !contains(event.Tags, t) {
			return fmt.Errorf("the tag %v is not present in the event's tags:\n event %v", t, event)
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
