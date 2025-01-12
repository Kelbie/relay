package dvm

import (
	"reflect"
	"relay/pkg/response"
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

func TestErrorEvent(t *testing.T) {
	testCases := []struct {
		name          string
		err           error
		req           *nostr.Event
		expectedEvent *nostr.Event
	}{
		{
			name: "nil req",
			err:  response.ErrBadlyFormattedTag,
			req:  nil,
			expectedEvent: &nostr.Event{
				Content: "",
				Kind:    KindDVMError,
				Tags: nostr.Tags{
					{"e", ""},
					{"p", ""},
					{"status", "error", response.ErrBadlyFormattedTag.Error()},
				},
			},
		},
		{
			name: "empty req",
			err:  response.ErrBadlyFormattedTag,
			req:  &nostr.Event{},
			expectedEvent: &nostr.Event{
				Content: "",
				Kind:    KindDVMError,
				Tags: nostr.Tags{
					{"e", ""},
					{"p", ""},
					{"status", "error", response.ErrBadlyFormattedTag.Error()},
				},
			},
		},
		{
			name: "nil error",
			err:  nil,
			req: &nostr.Event{
				ID:     "xxx",
				PubKey: fran,
			},
			expectedEvent: &nostr.Event{
				Content: "",
				Kind:    KindDVMError,
				Tags: nostr.Tags{
					{"e", "xxx"},
					{"p", fran},
					{"status", "error", ""},
				},
			},
		},
		{
			name: "valid",
			err:  response.ErrBadlyFormattedTag,
			req: &nostr.Event{
				ID:     "xxx",
				PubKey: fran,
			},
			expectedEvent: &nostr.Event{
				Content: "",
				Kind:    KindDVMError,
				Tags: nostr.Tags{
					{"e", "xxx"},
					{"p", fran},
					{"status", "error", response.ErrBadlyFormattedTag.Error()},
				},
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			event := ErrorEvent(test.err, test.req)
			test.expectedEvent.CreatedAt = nostr.Now()

			if !reflect.DeepEqual(event, test.expectedEvent) {
				t.Fatalf("ErrorEvent(): expected %v, got %v", test.expectedEvent, event)
			}
		})
	}
}

func TestResponseEvent(t *testing.T) {
	testCases := []struct {
		name          string
		res           []response.T
		req           *nostr.Event
		expectedEvent *nostr.Event
	}{
		{
			name: "nil req",
			res:  []response.T{{Pubkey: "abc", Rank: 0.7}},
			req:  nil,
			expectedEvent: &nostr.Event{
				Content: "[{\"pubkey\":\"abc\",\"rank\":0.7}]",
				Kind:    1000,
				Tags: nostr.Tags{
					{"e", ""},
					{"p", ""},
				},
			},
		},
		{
			name: "nil res",
			res:  nil,
			req: &nostr.Event{
				ID:     "xxx",
				PubKey: fran,
				Kind:   KindSortAuthors,
			},
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
			name: "valid",
			res:  []response.T{{Pubkey: "abc", Rank: 0.1}, {Pubkey: "123", Rank: 0.2}},
			req: &nostr.Event{
				ID:     "xxx",
				PubKey: fran,
				Kind:   KindSortAuthors,
			},
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
			event := ResponseEvent(test.res, test.req)
			test.expectedEvent.CreatedAt = nostr.Now()

			if !reflect.DeepEqual(event, test.expectedEvent) {
				t.Fatalf("ResponseEvent(): expected %v, got %v", test.expectedEvent, event)
			}
		})
	}
}
