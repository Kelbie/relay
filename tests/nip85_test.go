package tests

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/relay/pkg/nip85"
)

func TestNIP85_RankProfiles(t *testing.T) {
	filter := nostr.Filter{
		Kinds:   []int{nip85.Kind},
		Authors: []string{relayPubkey},
		Tags: nostr.TagMap{
			"d": {calle, calle, fran, randomKey, "zzz"}, // calle is duplicate, zzz is invalid
		},
	}

	expectedPubkeys := []string{calle, fran, randomKey, "zzz"}

	events, err := nip85Response(filter, localhost)
	if err != nil {
		t.Fatal(err)
	}

	for _, ev := range events {
		if err := checkFormat(&ev, nip85.Kind, nostr.Tags{{"d"}, {"rank"}}); err != nil {
			t.Fatalf("wrong event format: %v", err)
		}
	}

	pubkeys := extractNIP85Pubkeys(events)
	if !slices.Equal(pubkeys, expectedPubkeys) {
		PrintDifference(t, pubkeys, expectedPubkeys)
	}
}

func nip85Response(filter nostr.Filter, relayURL string) ([]nostr.Event, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	relay, err := nostr.RelayConnect(ctx, relayURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", relayURL, err)
	}

	ch, err := relay.QueryEvents(ctx, filter)
	if err != nil {
		return nil, err
	}

	var events []nostr.Event
	for ev := range ch {
		events = append(events, *ev)
	}
	return events, nil
}

func extractNIP85Pubkeys(events []nostr.Event) []string {
	pubkeys := make([]string, 0, len(events))
	for _, ev := range events {
		if d := ev.Tags.Find("d"); d != nil {
			pubkeys = append(pubkeys, d[1])
		}
	}
	return pubkeys
}
