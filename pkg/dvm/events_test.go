package dvm

import (
	"reflect"
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

func TestResponseEvent(t *testing.T) {
	tests := []struct {
		name     string
		res      Response
		rec      Record
		expected *nostr.Event
	}{
		{
			name: "empty res",
			res:  Response{},
			rec:  Record{ID: "xxx", Kind: KindSortProfiles, Pubkey: fran},
			expected: &nostr.Event{
				Content:   "[]",
				CreatedAt: nostr.Now(),
				Kind:      KindSortProfiles + 1000,
				Tags:      nostr.Tags{{"e", "xxx"}, {"p", fran}},
			},
		},
		{
			name: "response from empty ranking and extras",
			res:  NewResponse(nil, nil),
			rec:  Record{ID: "xxx", Kind: KindSortProfiles, Pubkey: fran},
			expected: &nostr.Event{
				Content:   "[]",
				CreatedAt: nostr.Now(),
				Kind:      KindSortProfiles + 1000,
				Tags:      nostr.Tags{{"e", "xxx"}, {"p", fran}},
			},
		},
		{
			name: "valid",
			res: Response{
				{Pubkey: "abc", Rank: 0.1, Extra: Extra{Follows: intPtr(69), Followers: intPtr(420)}},
				{Pubkey: "123", Rank: 0.2},
			},
			rec: Record{ID: "xxx", Kind: KindSortProfiles, Pubkey: fran},
			expected: &nostr.Event{
				Content:   "[{\"pubkey\":\"abc\",\"rank\":0.1,\"follows\":69,\"followers\":420},{\"pubkey\":\"123\",\"rank\":0.2}]",
				CreatedAt: nostr.Now(),
				Kind:      KindSortProfiles + 1000,
				Tags:      nostr.Tags{{"e", "xxx"}, {"p", fran}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := ResponseEvent(test.res, test.rec)
			if !reflect.DeepEqual(event, test.expected) {
				t.Fatalf("ResponseEvent(): expected %v, got %v", test.expected, event)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}
