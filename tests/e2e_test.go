// these are integration tests that require a Redis instance, that can be obtained by running the crawler for about 10 minutes.
package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vertex-lab/crawler_v2/pkg/pipe"
	"github.com/vertex-lab/relay/pkg/dvm"
	"github.com/vertex-lab/relay/pkg/rate"

	"github.com/nbd-wtf/go-nostr"
)

var (
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

	vertexURL     string = "wss://relay.vertexlab.io"
	localhost     string = "http://localhost:3334"
	defaultRelays        = []string{"wss://relay.primal.net", "wss://relay.nostr.band", "wss://relay.damus.io"}

	sk = "140494e2df64262cf14849db6e6e5333bca3e1d465cdd1acf2329a20a11b0b9c"
	pk = "3b0bc9a352e3b471b39879892c2116c1dc70f2aaff374f3cd01473ce19b2dcb4"
)

func init() {
	redis := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	limiter, err := rate.NewLimiter(redis, rate.NoRefill)
	if err != nil {
		log.Printf("init: failed to create limiter: %v", err)
	}

	if _, err := limiter.TopUp(pk, 100); err != nil {
		log.Printf("init: failed to top-up: %v", err)
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

	expectedTags := nostr.Tags{{"e", req.ID}, {"p", pk}, {"sort", dvm.Global}, {"nodes"}}
	expectedKind := req.Kind + 1000

	res, err := dvmResponse(req, localhost)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkFormat(res, expectedKind, expectedTags); err != nil {
		t.Fatalf("the format of the response is wrong: %v", err)
	}

	var response dvm.Response
	if err := json.Unmarshal([]byte(res.Content), &response); err != nil {
		t.Fatalf("failed to unmarshal the DVM response content: %v", err)
	}

	pubkeys := response.Pubkeys()
	if pubkeys[0] != fran {
		t.Errorf("the first pubkey should be the target %s", fran)
	}

	if err := checkPubkeysFollowTarget(pubkeys[1:], fran); err != nil {
		t.Fatal(err)
	}
}

func TestDVM_RankProfiles(t *testing.T) {
	// step 1. publishing a DVM request
	req := &nostr.Event{
		Kind:      dvm.KindRankProfiles,
		CreatedAt: nostr.Now(),
		Tags: nostr.Tags{
			{"param", "source", odell},
			{"param", "target", calle},
			{"param", "target", calle}, // duplicate
			{"param", "target", fran},
			{"param", "target", randomKey},
			{"param", "target", "zzz"}, // invalid key
		},
	}

	if err := req.Sign(sk); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	expectedSorted := []string{calle, fran, randomKey, "zzz"}
	expectedTags := nostr.Tags{{"e", req.ID}, {"p", pk}, {"sort", dvm.Global}, {"nodes"}}
	expectedKind := req.Kind + 1000

	res, err := dvmResponse(req, localhost)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkFormat(res, expectedKind, expectedTags); err != nil {
		t.Fatalf("the format of the response is wrong: %v", err)
	}

	var response dvm.Response
	if err := json.Unmarshal([]byte(res.Content), &response); err != nil {
		t.Fatalf("failed to unmarshal the DVM response content: %v", err)
	}

	sorted := response.Pubkeys()
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
	expectedTags := nostr.Tags{{"e", req.ID}, {"p", pk}, {"sort", dvm.Global}, {"nodes"}}
	expectedKind := req.Kind + 1000

	res, err := dvmResponse(req, localhost)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkFormat(res, expectedKind, expectedTags); err != nil {
		t.Fatalf("the format of the response is wrong: %v", err)
	}

	var response dvm.Response
	if err := json.Unmarshal([]byte(res.Content), &response); err != nil {
		t.Fatalf("failed to unmarshal the DVM response content: %v", err)
	}

	recommendations := response.Pubkeys()
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
	expectedTags := nostr.Tags{{"e", req.ID}, {"p", pk}, {"sort", dvm.Global}, {"nodes"}}
	expectedKind := req.Kind + 1000

	res, err := dvmResponse(req, localhost)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkFormat(res, expectedKind, expectedTags); err != nil {
		t.Fatalf("the format of the response is wrong: %v", err)
	}

	var response dvm.Response
	if err := json.Unmarshal([]byte(res.Content), &response); err != nil {
		t.Fatalf("failed to unmarshal the DVM response content: %v", err)
	}

	jacks := response.Pubkeys()
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

// dvmResponse connects to the relay, send the request and fetches the response using the request ID.
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

	filter := nostr.Filter{
		Kinds: []int{req.Kind + 1000, dvm.KindDVMError},
		Tags:  nostr.TagMap{"e": {req.ID}},
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

// checkPubkeysFollowTarget checks that each of the pubkeys listed in the response follows the target.
func checkPubkeysFollowTarget(pubkeys []string, target string) error {
	for _, pk := range pubkeys {
		if !nostr.IsValidPublicKey(pk) {
			return fmt.Errorf("%s is not a valid public key", pk)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool := nostr.NewSimplePool(ctx)
	filter := nostr.Filter{
		Authors: pubkeys,
		Kinds:   []int{nostr.KindFollowList},
	}

	newest := make(map[string]*nostr.Event, len(pubkeys))
	for event := range pool.FetchMany(ctx, defaultRelays, filter) {
		if _, exists := newest[event.PubKey]; !exists {
			newest[event.PubKey] = event.Event
			continue
		}

		if event.CreatedAt > newest[event.PubKey].CreatedAt {
			newest[event.PubKey] = event.Event
		}
	}

	if len(newest) != len(pubkeys) {
		return fmt.Errorf("expected to receive %d follow-lists, got %d", len(pubkeys), len(newest))
	}

	for pubkey, event := range newest {
		follows := pipe.ParsePubkeys(event)
		if !slices.Contains(follows, target) {
			return fmt.Errorf("%v doesn't follow %v, but the response showed otherwise", pubkey, target)
		}
	}

	return nil
}

// checkFormat checks that the event's kind and tags match the expected values.
func checkFormat(event *nostr.Event, kind int, tags nostr.Tags) error {
	if event.Kind != kind {
		return fmt.Errorf("expected kind %d, got %d:\n event %v", kind, event.Kind, event)
	}

	for _, tag := range tags {
		if !contains(event.Tags, tag) {
			return fmt.Errorf("expected tags %v, got %v:\n event %v", tags, event.Tags, event)
		}
	}

	return nil
}

// contains returns whether tags contains the specified tag.
// Tag might be strictly contained in tags, for example:
// {{"a", "b"}...} contains {"a"} and {"a", "b"}
func contains(tags nostr.Tags, tag nostr.Tag) bool {
outer:
	for _, t := range tags {
		if len(t) < len(tag) {
			continue outer
		}

		for i := range tag {
			if t[i] != tag[i] {
				continue outer
			}
		}
		return true
	}
	return false
}
