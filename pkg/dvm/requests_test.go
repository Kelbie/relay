package dvm

import (
	"errors"
	"reflect"
	"testing"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/relay/pkg/core"
)

func TestParseVerifyReputation(t *testing.T) {
	tests := []struct {
		name     string
		request  *nostr.Event
		expected *core.VerifyReputationArgs
		err      error
	}{
		{
			name: "invalid limit",
			request: &nostr.Event{
				Kind: KindVerifyReputation,
				Tags: nostr.Tags{{"param", "limit", "sixty-nine"}},
			},
			err: core.ErrInvalidLimit,
		},
		{
			name: "unsupported param",
			request: &nostr.Event{
				Kind: KindVerifyReputation,
				Tags: nostr.Tags{{"param", "search", "sixty-nine"}},
			},
			err: core.ErrUnsuportedVerifyReputation,
		},
		{
			name: "valid",
			request: &nostr.Event{
				Kind: KindVerifyReputation,
				Tags: nostr.Tags{
					{"param", "source", "pip"},
					{"param", "sort", "algo"},
					{"param", "limit", "9"},
					{"param", "target", "calle"},
					{"client", "coracle"}, // ignored tag
				},
			},
			expected: &core.VerifyReputationArgs{
				Algorithm: core.Algorithm{
					Sort:   "algo",
					Source: "pip",
				},
				Target: "calle",
				Limit:  9,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args, err := parseVerifyReputation(test.request)
			if !errors.Is(err, test.err) {
				t.Fatalf("expected error %v, got %v", test.err, err)
			}

			if err == nil && !reflect.DeepEqual(args, test.expected) {
				t.Errorf("expected args %v, got %v", test.expected, args)
			}
		})
	}
}

func TestParseRecommendFollows(t *testing.T) {
	tests := []struct {
		name     string
		request  *nostr.Event
		expected *core.RecommendFollowsArgs
		err      error
	}{
		{
			name: "invalid limit",
			request: &nostr.Event{
				Kind: KindRecommendFollows,
				Tags: nostr.Tags{{"param", "limit", "sixty-nine"}},
			},
			err: core.ErrInvalidLimit,
		},
		{
			name: "unsupported param",
			request: &nostr.Event{
				Kind: KindRecommendFollows,
				Tags: nostr.Tags{{"param", "target", "calle"}},
			},
			err: core.ErrUnsuportedRecommendFollows,
		},
		{
			name: "valid",
			request: &nostr.Event{
				Kind: KindRecommendFollows,
				Tags: nostr.Tags{
					{"param", "source", "pip"},
					{"param", "sort", "algo"},
					{"param", "limit", "9"},
					{"client", "coracle"}, // ignored tag
				},
			},
			expected: &core.RecommendFollowsArgs{
				Algorithm: core.Algorithm{
					Sort:   "algo",
					Source: "pip",
				},
				Limit: 9,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args, err := parseRecommendFollows(test.request)
			if !errors.Is(err, test.err) {
				t.Fatalf("expected error %v, got %v", test.err, err)
			}

			if err == nil && !reflect.DeepEqual(args, test.expected) {
				t.Errorf("expected args %v, got %v", test.expected, args)
			}
		})
	}
}

func TestParseRankProfiles(t *testing.T) {
	tests := []struct {
		name     string
		request  *nostr.Event
		expected *core.RankProfilesArgs
		err      error
	}{
		{
			name: "invalid limit",
			request: &nostr.Event{
				Kind: KindRankProfiles,
				Tags: nostr.Tags{{"param", "limit", "sixty-nine"}},
			},
			err: core.ErrInvalidLimit,
		},
		{
			name: "unsupported param",
			request: &nostr.Event{
				Kind: KindRankProfiles,
				Tags: nostr.Tags{{"param", "search", "calle"}},
			},
			err: core.ErrUnsuportedRankProfiles,
		},
		{
			name: "valid",
			request: &nostr.Event{
				Kind: KindRankProfiles,
				Tags: nostr.Tags{
					{"param", "source", "pip"},
					{"param", "sort", "algo"},
					{"param", "target", "calle"},
					{"param", "target", "jack"},
					{"param", "target", "odell"},
					{"param", "limit", "9"},
					{"client", "coracle"}, // ignored tag
				},
			},
			expected: &core.RankProfilesArgs{
				Algorithm: core.Algorithm{
					Sort:   "algo",
					Source: "pip",
				},
				Targets: []string{"calle", "jack", "odell"},
				Limit:   9,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args, err := parseRankProfiles(test.request)
			if !errors.Is(err, test.err) {
				t.Fatalf("expected error %v, got %v", test.err, err)
			}

			if err == nil && !reflect.DeepEqual(args, test.expected) {
				t.Errorf("expected args %v, got %v", test.expected, args)
			}
		})
	}
}

func TestParseSearchProfiles(t *testing.T) {
	tests := []struct {
		name     string
		request  *nostr.Event
		expected *core.SearchProfilesArgs
		err      error
	}{
		{
			name: "invalid limit",
			request: &nostr.Event{
				Kind: KindSearchProfiles,
				Tags: nostr.Tags{{"param", "limit", "sixty-nine"}},
			},
			err: core.ErrInvalidLimit,
		},
		{
			name: "unsupported param",
			request: &nostr.Event{
				Kind: KindSearchProfiles,
				Tags: nostr.Tags{{"param", "target", "calle"}},
			},
			err: core.ErrUnsuportedRankProfiles,
		},
		{
			name: "valid",
			request: &nostr.Event{
				Kind: KindSearchProfiles,
				Tags: nostr.Tags{
					{"param", "source", "pip"},
					{"param", "sort", "algo"},
					{"param", "search", "nostr"},
					{"param", "limit", "9"},
					{"client", "coracle"}, // ignored tag
				},
			},
			expected: &core.SearchProfilesArgs{
				Algorithm: core.Algorithm{
					Sort:   "algo",
					Source: "pip",
				},
				Search: "nostr",
				Limit:  9,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args, err := parseSearchProfiles(test.request)
			if !errors.Is(err, test.err) {
				t.Fatalf("expected error %v, got %v", test.err, err)
			}

			if err == nil && !reflect.DeepEqual(args, test.expected) {
				t.Errorf("expected args %v, got %v", test.expected, args)
			}
		})
	}
}
