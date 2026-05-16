package api

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/rely/v2"
)

var (
	validEvent       = []byte(`{"kind":1,"id":"dc90c95f09947507c1044e8f48bcf6350aa6bff1507dd4acfc755b9239b5c962","pubkey":"3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d","created_at":1644271588,"tags":[],"content":"now that https://blueskyweb.org/blog/2-7-2022-overview was announced we can stop working on nostr?","sig":"230e9d8f0ddaf7eb70b5f7741ccfa37e87a455c9a469282e3464e2052d3192cd63a167e196e381ef9d7e69e9ea43af2443b839974dc85d8aaab9efe1d9296524"}`)
	invalidID        = []byte(`{"kind":1,"id":"dc90c95f099----invalidated------0aa6bff1507dd4acfc755b9239b5c962","pubkey":"3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d","created_at":1644271588,"tags":[],"content":"now that https://blueskyweb.org/blog/2-7-2022-overview was announced we can stop working on nostr?","sig":"230e9d8f0ddaf7eb70b5f7741ccfa37e87a455c9a469282e3464e2052d3192cd63a167e196e381ef9d7e69e9ea43af2443b839974dc85d8aaab9efe1d9296524"}`)
	invalidSignature = []byte(`{"kind":1,"id":"dc90c95f09947507c1044e8f48bcf6350aa6bff1507dd4acfc755b9239b5c962","pubkey":"3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d","created_at":1644271588,"tags":[],"content":"now that https://blueskyweb.org/blog/2-7-2022-overview was announced we can stop working on nostr?","sig":"230e9d8f0ddaf7eb70b5f7741ccfa37e87a455c9a469282e------------invalidated-------------69e9ea43af2443b839974dc85d8aaab9efe1d9296524"}`)
)

func TestParseDVMs(t *testing.T) {
	tests := []struct {
		name     string
		body     io.Reader
		expected *nostr.Event
		err      error
	}{
		{
			name: "invalid json",
			body: bytes.NewReader([]byte("invalid json")),
			err:  ErrInvalidEventJSON,
		},
		{
			name: "invalid id",
			body: bytes.NewReader(invalidID),
			err:  rely.ErrInvalidEventID,
		},
		{
			name: "invalid signature",
			body: bytes.NewReader(invalidSignature),
			err:  rely.ErrInvalidEventSig,
		},
		{
			name: "valid",
			body: bytes.NewReader(validEvent),
			expected: &nostr.Event{
				Kind:      1,
				ID:        "dc90c95f09947507c1044e8f48bcf6350aa6bff1507dd4acfc755b9239b5c962",
				PubKey:    "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
				CreatedAt: 1644271588,
				Tags:      nostr.Tags{},
				Content:   "now that https://blueskyweb.org/blog/2-7-2022-overview was announced we can stop working on nostr?",
				Sig:       "230e9d8f0ddaf7eb70b5f7741ccfa37e87a455c9a469282e3464e2052d3192cd63a167e196e381ef9d7e69e9ea43af2443b839974dc85d8aaab9efe1d9296524",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest("POST", "/api/v1/dvms", test.body)
			if err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			event, err := ParseDVM(request)
			if !errors.Is(err, test.err) {
				t.Fatalf("expected error %v, got %v", test.err, err)
			}

			if !reflect.DeepEqual(event, test.expected) {
				t.Fatalf("expecte event %v, got %v", test.expected, event)
			}
		})
	}
}
