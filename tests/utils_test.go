package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip11"
	"github.com/redis/go-redis/v9"
	"github.com/vertex-lab/relay/pkg/core"
	"github.com/vertex-lab/relay/pkg/credits"
)

// pubkeys used in tests
var (
	jack_dorsey  string = "82341f882b6eabcd2ba7f1ef90aad961cf074af15b9ef44a09f9d2a8fbfbe6a2"
	jack_mallers string = "c4eabae1be3cf657bc1855ee05e69de9f059cb7a059227168b80b89761cbc4e0"
	jack_spirko  string = "a1fc5dfd7ffcf563c89155b466751b580d115e136e2f8c90e8913385bbedb1cf"
	damus        string = "3efdaebb1d8923ebd99c9e7ace3b4194ab45512e2be79c1b7d68d9243e0d2681"
	jb55         string = "32e1827635450ebb3c5a7d12c1f8e7b2b514439ac10a67eef3d9fd9c5c68e245"
	snowden      string = "84dee6e676e5bb67b4ad4e042cf70cbd8681155db535942fcc6a0533858a7240"
	fran         string = "726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11"
	odell        string = "04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9"
	fiatjaf      string = "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"
	marty        string = "472f440f29ef996e92a186b8d320ff180c855903882e59d50de1b8bd5669301e"
	derek        string = "3f770d65d3a764a9c5cb503ae123e62ec7598ad035d836e2a810f3877a745b24"
	kieran       string = "63fe6318dc58583cfe16810f86dd09e18bfd76aabc24a0081ce2856f330504ed"
	calle        string = "50d94fc2d8580c682b071a542f8b1e31a200b0508bab95a33bef0855df281d63"
	pip          string = "f683e87035f7ad4f44e0b98cfbd9537e16455a92cd38cefc4cb31db7557f5ef2"
	randomKey    string = "d5ad3d3115d9fa07500b06ccd0b9605d9888a206acba20a1e2e681ec29109387"
)

var (
	vertexURL string = "wss://relay.vertexlab.io"
	port      string = "3334"

	sk = "140494e2df64262cf14849db6e6e5333bca3e1d465cdd1acf2329a20a11b0b9c"
	pk = "3b0bc9a352e3b471b39879892c2116c1dc70f2aaff374f3cd01473ce19b2dcb4"

	relayPubkey string
)

func init() {
	redis := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	creditManager, err := credits.NewManager(redis, credits.NoRefill)
	if err != nil {
		log.Printf("init: failed to create limiter: %v", err)
	}

	if _, err := creditManager.TopUp(pk, 100); err != nil {
		log.Printf("init: failed to top-up: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	info, err := nip11.Fetch(ctx, relayURL)
	if err != nil {
		log.Printf("init: failed to fetch relay info: %v", err)
	} else {
		relayPubkey = info.PubKey
	}
}

func PrintDifference(t *testing.T, pubkeys, expected []string) {
	t.Errorf("pubkeys don't match the expected ones")
	t.Errorf("pubkeys:")
	for i, pk := range pubkeys {
		t.Errorf("%d) %s", i, pk)
	}
	t.Errorf("expected:")
	for i, pk := range expected {
		t.Errorf("%d) %s", i, pk)
	}
}

func ExtractPubkeys(dvmContent string) ([]string, error) {
	var profiles []core.Profile
	if err := json.Unmarshal([]byte(dvmContent), &profiles); err != nil {
		return nil, fmt.Errorf("failed to unmarshal the dvm content: %w", err)
	}

	pubkeys := make([]string, len(profiles))
	for i := range profiles {
		pubkeys[i] = profiles[i].Pubkey
	}
	return pubkeys, nil
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
