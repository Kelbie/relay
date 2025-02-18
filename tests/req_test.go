package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/vertex-lab/relay/pkg/dvm"

	"github.com/nbd-wtf/go-nostr"
)

func TestREQ_VerifyReputation(t *testing.T) {
	req := nostr.Filter{
		Kinds:  []int{dvm.KindVerifyReputation + 1000, dvm.KindDVMError},
		Search: "{\"source\":\"04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9\", \"targets\":[\"726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11\"]}",
	}

	expectedKind := req.Kinds[0]
	expectedTags := nostr.Tags{}

	res, err := reqResponse(req, localhost)
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

	pubkeys, _ := ranks.Unpack()
	if err := checkPubkeysFollowTarget(pubkeys, fran); err != nil {
		t.Fatal(err)
	}
}

// ------------------------------------HELPERS---------------------------------

// reqResponse() connects to the relay, send the REQ.
func reqResponse(req nostr.Filter, relayURL string) (res *nostr.Event, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	relay, err := nostr.RelayConnect(ctx, relayURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", relayURL, err)
	}

	var counter int
	ch, err := relay.QueryEvents(ctx, req)
	for event := range ch {
		res = event
		counter++
	}

	if counter != 1 {
		return nil, fmt.Errorf("expected exactly one response, got %v", counter)
	}

	return res, nil
}
