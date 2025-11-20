package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/relay/pkg/core"
	"github.com/vertex-lab/relay/pkg/credits"
	"github.com/vertex-lab/relay/pkg/dvm"
)

var dvmEndpoint = localhost + "/api/v1/dvms"

func TestAPI_CreditManagement(t *testing.T) {
	requestuest := &nostr.Event{
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

	if err := requestuest.Sign(sk); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	response, err := apiDVM(requestuest, dvmEndpoint)
	if err != nil {
		t.Fatal(err)
	}

	expectedKind := dvm.KindDVMError
	expectedTags := nostr.Tags{
		{"e", requestuest.ID},
		{"p", pk},
		{"status", "error", credits.ErrInsufficientCredits.Error()},
	}

	err = checkFormat(response, expectedKind, expectedTags)
	if err != nil {
		t.Fatalf("the format of the response is wrong: %v", err)
	}
}

func TestAPI_VerifyReputation(t *testing.T) {
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

	response, err := apiDVM(request, dvmEndpoint)
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

func TestAPI_RankProfiles(t *testing.T) {
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

	response, err := apiDVM(request, dvmEndpoint)
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

func TestAPI_RecommendFollows(t *testing.T) {
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

	response, err := apiDVM(request, dvmEndpoint)
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

	// This list is dependent on the specific database. If it doesn't return the exact same list
	// then do some manual checking to make sure it makes sense.
	if !slices.Equal(pubkeys, expectedPubkeys) {
		PrintDifference(t, pubkeys, expectedPubkeys)
	}
}

func TestAPI_SearchProfiles(t *testing.T) {
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

	response, err := apiDVM(request, dvmEndpoint)
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

	// This list is dependent on the specific database. If it doesn't return the exact same list
	// then do some manual checking to make sure it makes sense.
	if !slices.Equal(pubkeys, expectedPubkeys) {
		PrintDifference(t, pubkeys, expectedPubkeys)
	}
}

// apiDVM construct a POST requestuest with body the JSON of the specified nostr requestuest.
func apiDVM(requestuestEvent *nostr.Event, url string) (*nostr.Event, error) {
	requestuestBody, err := json.Marshal(requestuestEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal requestuest event: %v", err)
	}

	requestuest, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestuestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP requestuest: %v", err)
	}
	requestuest.Header.Set("Content-Type", "application/json")

	response, err := apiResponse(requestuest)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	if response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API returned status code %d\n body: %v", response.StatusCode, string(body))
	}

	responseEvent := &nostr.Event{}
	if err := json.Unmarshal(body, responseEvent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}
	return responseEvent, nil
}

// apiResponse sends the provided *http.Request to the specified URL
// and returns the *http.Response.
func apiResponse(requestuest *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	requestuest = requestuest.WithContext(ctx)

	client := &http.Client{}
	response, err := client.Do(requestuest)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP requestuest: %w", err)
	}
	return response, nil
}
