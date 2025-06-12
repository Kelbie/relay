package dvm

import (
	"reflect"
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

const maxDist float64 = 0.002

func TestResponseEvent(t *testing.T) {
	record := Record{ID: "xxx", Kind: KindRankProfiles, Pubkey: fran, Timestamp: 420, Nodes: 69}
	tests := []struct {
		name     string
		res      Response
		req      *Request
		expected *nostr.Event
	}{
		{
			name: "empty res",
			res:  Response{},
			req:  &Request{Record: record, Algorithm: Algorithm{Sort: Global}},
			expected: &nostr.Event{
				Content:   "[]",
				CreatedAt: 420,
				Kind:      KindRankProfiles + 1000,
				Tags:      nostr.Tags{{"e", "xxx"}, {"p", fran}, {"sort", Global}, {"nodes", "69"}},
			},
		},
		{
			name: "response from empty ranking and extras",
			res:  NewResponse(nil),
			req:  &Request{Record: record, Algorithm: Algorithm{Sort: Global}},
			expected: &nostr.Event{
				Content:   "[]",
				CreatedAt: 420,
				Kind:      KindRankProfiles + 1000,
				Tags:      nostr.Tags{{"e", "xxx"}, {"p", fran}, {"sort", Global}, {"nodes", "69"}},
			},
		},
		{
			name: "valid",
			res: Response{
				{Pubkey: "abc", Rank: 0.1, Extra: Extra{Follows: pointer(69), Followers: pointer(420)}},
				{Pubkey: "123", Rank: 0.2},
			},
			req: &Request{Record: record, Algorithm: Algorithm{Sort: Global}},
			expected: &nostr.Event{
				Content:   "[{\"pubkey\":\"abc\",\"rank\":0.1,\"follows\":69,\"followers\":420},{\"pubkey\":\"123\",\"rank\":0.2}]",
				CreatedAt: 420,
				Kind:      KindRankProfiles + 1000,
				Tags:      nostr.Tags{{"e", "xxx"}, {"p", fran}, {"sort", Global}, {"nodes", "69"}},
			},
		},
		{
			name: "valid personalized",
			res: Response{
				{Pubkey: "abc", Rank: 0.1, Extra: Extra{Follows: pointer(69), Followers: pointer(420)}},
				{Pubkey: "123", Rank: 0.2},
			},
			req: &Request{Record: record, Algorithm: Algorithm{Sort: Personalized, Source: pip}},
			expected: &nostr.Event{
				Content:   "[{\"pubkey\":\"abc\",\"rank\":0.1,\"follows\":69,\"followers\":420},{\"pubkey\":\"123\",\"rank\":0.2}]",
				CreatedAt: 420,
				Kind:      KindRankProfiles + 1000,
				Tags:      nostr.Tags{{"e", "xxx"}, {"p", fran}, {"sort", Personalized}, {"source", pip}, {"nodes", "69"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := ResponseEvent(test.res, test.req)
			if !reflect.DeepEqual(event, test.expected) {
				t.Fatalf("ResponseEvent(): expected %v, got %v", test.expected, event)
			}
		})
	}
}

func TestEscapeFTS5(t *testing.T) {
	tests := []struct {
		term     string
		expected string
	}{
		{term: `jack`, expected: `"jack"`},
		{term: `don't`, expected: `"don't"`},
		{term: `she said "get out!"`, expected: `"she said ""get out!"""`},
	}

	for _, test := range tests {
		t.Run(test.term, func(t *testing.T) {
			str := escapeFTS5(test.term)
			if str != test.expected {
				t.Fatalf(`expected term '%s', got '%s'`, test.expected, str)
			}
		})
	}
}

func pointer[T any](t T) *T {
	return &t
}
