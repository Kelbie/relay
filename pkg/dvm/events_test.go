package dvm

import (
	"reflect"
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

func TestResponseEvent(t *testing.T) {
	testCases := []struct {
		name          string
		res           PubkeyRanks
		rec           Record
		expectedEvent *nostr.Event
	}{
		{
			name: "nil res",
			rec:  Record{ID: "xxx", Kind: KindSortProfiles, Pubkey: fran},
			expectedEvent: &nostr.Event{
				Content: "[]",
				Kind:    KindSortProfiles + 1000,
				Tags:    nostr.Tags{{"e", "xxx"}, {"p", fran}},
			},
		},
		{
			name: "empty res",
			res:  PubkeyRanks{},
			rec:  Record{ID: "xxx", Kind: KindSortProfiles, Pubkey: fran},
			expectedEvent: &nostr.Event{
				Content: "[]",
				Kind:    KindSortProfiles + 1000,
				Tags:    nostr.Tags{{"e", "xxx"}, {"p", fran}},
			},
		},
		{
			name: "valid",
			res:  PubkeyRanks{{Key: "abc", Val: 0.1}, {Key: "123", Val: 0.2}},
			rec:  Record{ID: "xxx", Kind: KindSortProfiles, Pubkey: fran},
			expectedEvent: &nostr.Event{
				Content: "[{\"pubkey\":\"abc\",\"rank\":0.1},{\"pubkey\":\"123\",\"rank\":0.2}]",
				Kind:    KindSortProfiles + 1000,
				Tags:    nostr.Tags{{"e", "xxx"}, {"p", fran}},
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			event := ResponseEvent(test.res, test.rec)
			test.expectedEvent.CreatedAt = nostr.Now()

			if !reflect.DeepEqual(event, test.expectedEvent) {
				t.Fatalf("ResponseEvent(): expected %v, got %v", test.expectedEvent, event)
			}
		})
	}
}
