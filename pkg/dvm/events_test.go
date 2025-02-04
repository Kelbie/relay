package dvm

import (
	"reflect"
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

func TestResponseEvent(t *testing.T) {
	testCases := []struct {
		name          string
		res           []RankResponse
		kind          int
		ID            string
		pubkey        string
		expectedEvent *nostr.Event
	}{
		{
			name:   "nil res",
			res:    nil,
			kind:   KindSortAuthors,
			ID:     "xxx",
			pubkey: fran,
			expectedEvent: &nostr.Event{
				Content: "null",
				Kind:    KindSortAuthors + 1000,
				Tags: nostr.Tags{
					{"e", "xxx"},
					{"p", fran},
				},
			},
		},
		{
			name:   "empty res",
			res:    []RankResponse{},
			kind:   KindSortAuthors,
			ID:     "xxx",
			pubkey: fran,
			expectedEvent: &nostr.Event{
				Content: "[]",
				Kind:    KindSortAuthors + 1000,
				Tags: nostr.Tags{
					{"e", "xxx"},
					{"p", fran},
				},
			},
		},
		{
			name:   "valid",
			res:    []RankResponse{{Pubkey: "abc", Rank: 0.1}, {Pubkey: "123", Rank: 0.2}},
			kind:   KindSortAuthors,
			ID:     "xxx",
			pubkey: fran,
			expectedEvent: &nostr.Event{
				Content: "[{\"pubkey\":\"abc\",\"rank\":0.1},{\"pubkey\":\"123\",\"rank\":0.2}]",
				Kind:    KindSortAuthors + 1000,
				Tags: nostr.Tags{
					{"e", "xxx"},
					{"p", fran},
				},
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			event := ResponseEvent(test.res, test.ID, test.pubkey, test.kind)
			test.expectedEvent.CreatedAt = nostr.Now()

			if !reflect.DeepEqual(event, test.expectedEvent) {
				t.Fatalf("ResponseEvent(): expected %v, got %v", test.expectedEvent, event)
			}
		})
	}
}
