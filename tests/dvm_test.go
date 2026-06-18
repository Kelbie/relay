// these are integration tests that require a Redis instance, that can be obtained by running the crawler for about 10 minutes.
package tests

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/vertex-lab/relay/pkg/core"
	"github.com/vertex-lab/relay/pkg/credits"
	"github.com/vertex-lab/relay/pkg/dvm"

	"github.com/nbd-wtf/go-nostr"
)

var relayURL = fmt.Sprintf("ws://localhost:%s", port)

func TestDVM_CreditManagement(t *testing.T) {
	request := &nostr.Event{
		Kind: dvm.KindVerifyReputation,
		Tags: nostr.Tags{
			{"param", "target", fran},
		},
	}

	sk := nostr.GeneratePrivateKey() // random key with no credits
	pk, err := nostr.GetPublicKey(sk)
	if err != nil {
		t.Fatalf("failed to get pk from sk: %v", err)
	}

	if err := request.Sign(sk); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	expectedKind := dvm.KindDVMError
	expectedTags := nostr.Tags{
		{"e", request.ID},
		{"p", pk},
		{"status", "error", credits.ErrInsufficientCredits.Error()},
	}

	response, err := dvmResponse(request, relayURL)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkFormat(response, expectedKind, expectedTags); err != nil {
		t.Fatalf("the format of the response is wrong: %v", err)
	}
}

func TestDVM_VerifyReputation(t *testing.T) {
	request := &nostr.Event{
		Kind:      dvm.KindVerifyReputation,
		CreatedAt: nostr.Now(),
		Tags: nostr.Tags{
			{"param", "source", odell},
			{"param", "target", fran},
			{"param", "limit", "3"},
		},
	}

	if err := request.Sign(sk); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	expectedPubkeys := []string{fran, odell, fiatjaf, marty}
	expectedTags := nostr.Tags{{"e", request.ID}, {"p", pk}, {"sort", core.Global}, {"nodes"}}
	expectedKind := request.Kind + 1000

	response, err := dvmResponse(request, relayURL)
	if err != nil {
		t.Fatal(err)
	}

	err = checkFormat(response, expectedKind, expectedTags)
	if err != nil {
		t.Fatalf("the format of the response is wrong: %v", err)
	}

	pubkeys, err := ExtractPubkeys(response.Content)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(pubkeys, expectedPubkeys) {
		PrintDifference(t, pubkeys, expectedPubkeys)
	}
}

func TestDVM_RankProfiles(t *testing.T) {
	request := &nostr.Event{
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

	if err := request.Sign(sk); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	expectedPubkeys := []string{calle, fran, randomKey, "zzz"}
	expectedTags := nostr.Tags{{"e", request.ID}, {"p", pk}, {"sort", core.Global}, {"nodes"}}
	expectedKind := request.Kind + 1000

	response, err := dvmResponse(request, relayURL)
	if err != nil {
		t.Fatal(err)
	}

	err = checkFormat(response, expectedKind, expectedTags)
	if err != nil {
		t.Fatalf("the format of the response is wrong: %v", err)
	}

	pubkeys, err := ExtractPubkeys(response.Content)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(pubkeys, expectedPubkeys) {
		PrintDifference(t, pubkeys, expectedPubkeys)
	}
}

func TestDVM_RecommendFollows(t *testing.T) {
	request := &nostr.Event{
		Kind:      dvm.KindRecommendFollows,
		CreatedAt: nostr.Now(),
		Tags: nostr.Tags{
			{"param", "source", randomKey},
			{"param", "limit", "3"},
		},
	}

	if err := request.Sign(sk); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	expectedPubkeys := []string{damus, jack_dorsey, jb55}
	expectedTags := nostr.Tags{{"e", request.ID}, {"p", pk}, {"sort", core.Global}, {"nodes"}}
	expectedKind := request.Kind + 1000

	response, err := dvmResponse(request, relayURL)
	if err != nil {
		t.Fatal(err)
	}

	err = checkFormat(response, expectedKind, expectedTags)
	if err != nil {
		t.Fatalf("the format of the response is wrong: %v", err)
	}

	pubkeys, err := ExtractPubkeys(response.Content)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(pubkeys, expectedPubkeys) {
		PrintDifference(t, pubkeys, expectedPubkeys)
	}
}

func TestDVM_SearchProfiles(t *testing.T) {
	request := &nostr.Event{
		Kind:      dvm.KindSearchProfiles,
		CreatedAt: nostr.Now(),
		Tags: nostr.Tags{
			{"param", "search", "jack"},
			{"param", "limit", "3"},
		},
	}

	if err := request.Sign(sk); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	expectedPubkeys := []string{jack_dorsey, jack_mallers, jack_spirko}
	expectedTags := nostr.Tags{{"e", request.ID}, {"p", pk}, {"sort", core.Global}, {"nodes"}}
	expectedKind := request.Kind + 1000

	response, err := dvmResponse(request, relayURL)
	if err != nil {
		t.Fatal(err)
	}

	err = checkFormat(response, expectedKind, expectedTags)
	if err != nil {
		t.Fatalf("the format of the response is wrong: %v", err)
	}

	pubkeys, err := ExtractPubkeys(response.Content)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(pubkeys, expectedPubkeys) {
		PrintDifference(t, pubkeys, expectedPubkeys)
	}
}

// dvmResponse connects to the relay, send the request and fetches the response using the request ID.
func dvmResponse(request *nostr.Event, relayURL string) (response *nostr.Event, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	relay, err := nostr.RelayConnect(ctx, relayURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", relayURL, err)
	}

	if err := relay.Publish(ctx, *request); err != nil {
		return nil, fmt.Errorf("failed to publish to %s: %v", relayURL, err)
	}

	filter := nostr.Filter{
		Kinds: []int{request.Kind + 1000, dvm.KindDVMError},
		Tags:  nostr.TagMap{"e": {request.ID}},
	}

	ch, err := relay.QueryEvents(ctx, filter)
	if err != nil {
		return nil, err
	}

	var counter int
	for event := range ch {
		response = event
		counter++
	}

	if counter != 1 {
		return nil, fmt.Errorf("expected exactly one response, got %v", counter)
	}
	return response, nil
}
