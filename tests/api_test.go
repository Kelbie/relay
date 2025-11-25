package tests

import (
	"bytes"
	"context"
	"encoding/base64"
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

func TestAPI_GetCredits(t *testing.T) {
	method := "GET"
	url := "http://localhost:3334/api/v1/credits"

	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("failed to create the request: %v", err)
	}

	auth := &nostr.Event{
		Kind:      nostr.KindHTTPAuth,
		CreatedAt: nostr.Now(),
		Tags: nostr.Tags{
			{"u", url},
			{"method", method},
		},
	}

	if err := auth.Sign(sk); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	json, err := auth.MarshalJSON()
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	header := "Nostr " + base64.RawURLEncoding.EncodeToString(json)
	request.Header["Authorization"] = []string{header}

	response, err := apiResponse(request)
	if err != nil {
		t.Fatalf("failed to call credits API: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Errorf("API returned status code %d", response.StatusCode)
		t.Fatalf("body: %v", string(body))
	}

	credits, err := parseEvent(response.Body)
	if err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}

	expectedKind := 22243
	expectedTags := nostr.Tags{{"credits"}, {"lastRequest"}}

	err = checkFormat(credits, expectedKind, expectedTags)
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

// apiDVM construct a POST request with body the JSON of the specified nostr request.
func apiDVM(requestuestEvent *nostr.Event, url string) (*nostr.Event, error) {
	requestuestBody, err := json.Marshal(requestuestEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request event: %v", err)
	}

	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestuestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := apiResponse(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API returned status code %d", response.StatusCode)
	}
	return parseEvent(response.Body)
}

func parseEvent(body io.ReadCloser) (*nostr.Event, error) {
	defer body.Close()

	bytes, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	responseEvent := &nostr.Event{}
	if err := json.Unmarshal(bytes, responseEvent); err != nil {
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
