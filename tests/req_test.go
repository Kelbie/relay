package tests

import (
	"context"
	"relay/pkg/dvm"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

func TestREQ_RelevantWhoFollow(t *testing.T) {
	// step 1. send the request to the relay
	req := nostr.Filter{
		Kinds:  []int{dvm.KindRelevantWhoFollow + 1000, dvm.KindDVMError},
		Search: "{\"source\":\"04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9\", \"targets\":[\"726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11\"]}",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	relay, err := nostr.RelayConnect(ctx, localhost)
	if err != nil {
		t.Fatalf("failed to connect to %v: %v", localhost, err)
	}

	var res *nostr.Event
	var counter int
	ch, err := relay.QueryEvents(ctx, req)
	for event := range ch {
		res = event
		counter++
	}

	if counter != 1 {
		t.Fatalf("expected exactly one event, got %v", counter)
	}

	// step 3. checking the response is consistent, meaning each pubkey follows target.
	if err := CheckResponseIsConsistent(res, fran); err != nil {
		t.Fatal(err)
	}
}
