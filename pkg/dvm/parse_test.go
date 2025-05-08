package dvm

import (
	"errors"
	"math/rand/v2"
	"reflect"
	"strconv"
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

// pubkeys for testing purposes
const (
	fran      string = "726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11"
	odell     string = "04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9"
	calle     string = "50d94fc2d8580c682b071a542f8b1e31a200b0508bab95a33bef0855df281d63"
	pip       string = "f683e87035f7ad4f44e0b98cfbd9537e16455a92cd38cefc4cb31db7557f5ef2"
	randomKey string = "d5ad3d3115d9fa07500b06ccd0b9605d9888a206acba20a1e2e681ec29109387"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name          string
		req           *nostr.Event
		expected      *Request
		expectedError error
	}{
		{
			name: "invalid limit",
			req: &nostr.Event{
				Kind: KindVerifyReputation,
				Tags: nostr.Tags{{"param", "limit", "sixty-nine"}},
			},
			expectedError: ErrInvalidLimit,
		},
		{
			name: "multiple source",
			req: &nostr.Event{
				Kind: KindVerifyReputation,
				Tags: nostr.Tags{{"param", "source", "aaa"}, {"param", "source", "bbb"}},
			},
			expectedError: ErrMultipleParams,
		},
		{
			name: "multiple sort",
			req: &nostr.Event{
				Kind: KindVerifyReputation,
				Tags: nostr.Tags{{"param", "sort", "aaa"}, {"param", "sort", "bbb"}},
			},
			expectedError: ErrMultipleParams,
		},
		{
			name: "multiple limit",
			req: &nostr.Event{
				Kind: KindVerifyReputation,
				Tags: nostr.Tags{{"param", "limit", "9"}, {"param", "limit", "11"}},
			},
			expectedError: ErrMultipleParams,
		},
		{
			name: "multiple search",
			req: &nostr.Event{
				Kind: KindVerifyReputation,
				Tags: nostr.Tags{{"param", "search", "jack"}, {"param", "search", "odell"}},
			},
			expectedError: ErrMultipleParams,
		},
		{
			name: "valid",
			req: &nostr.Event{
				Tags: nostr.Tags{
					{"param", "search", "jack"},
					{"param", "source", pip},
					{"param", "sort", Personalized},
					{"param", "limit", "9"},
					{"param", "target", calle},
					{"param", "target", odell},
					{"client", "coracle"}, // ignored tag
				},
			},
			expected: &Request{
				Record:    Record{Timestamp: nostr.Now()},
				Algorithm: Algorithm{Sort: Personalized, Source: pip},
				Search:    "jack",
				Targets:   []string{calle, odell},
				Limit:     9,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args, err := Parse(test.req)

			if !errors.Is(err, test.expectedError) {
				t.Fatalf("Parse(): expected %v, got %v", test.expectedError, err)
			}

			if !reflect.DeepEqual(args, test.expected) {
				t.Errorf("Parse(): expected %v, got %v", test.expected, args)
			}
		})
	}
}

func TestToVerifyReputation(t *testing.T) {
	tests := []struct {
		name          string
		req           *Request
		expected      *VerifyReputationArgs
		expectedError error
	}{
		{
			name:          "no targets",
			req:           &Request{},
			expectedError: ErrInvalidTarget,
		},
		{
			name:          "too many targets",
			req:           &Request{Targets: []string{pip, calle}},
			expectedError: ErrInvalidTarget,
		},
		{
			name:          "search",
			req:           &Request{Targets: []string{pip}, Search: "j"},
			expectedError: ErrParamNotSupported,
		},
		{
			name:          "invalid source key",
			req:           &Request{Algorithm: Algorithm{Sort: Personalized, Source: "abc"}, Targets: []string{pip}, Limit: 10},
			expectedError: ErrInvalidSource,
		},
		{
			name:          "invalid target key",
			req:           &Request{Algorithm: Algorithm{Sort: Global}, Targets: []string{"xxx"}, Limit: 10},
			expectedError: ErrInvalidTarget,
		},
		{
			name:          "negative limit",
			req:           &Request{Algorithm: Algorithm{Sort: Global}, Targets: []string{pip}, Limit: -1},
			expectedError: ErrInvalidLimit,
		},
		{
			name:          "limit too high",
			req:           &Request{Algorithm: Algorithm{Sort: Global}, Targets: []string{pip}, Limit: StandardMaxLimit + 1},
			expectedError: ErrInvalidLimit,
		},
		{
			name: "valid",
			req: &Request{
				Algorithm: Algorithm{Sort: Personalized, Source: odell},
				Targets:   []string{"npub176p7sup477k5738qhxx0hk2n0cty2k5je5uvalzvkvwmw4tltmeqw7vgup"},
				Limit:     69,
			},
			expected: &VerifyReputationArgs{
				Algorithm: Algorithm{Sort: Personalized, Source: odell},
				Target:    pip,
				Limit:     69,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args, err := test.req.ToVerifyReputationArgs()
			if !errors.Is(err, test.expectedError) {
				t.Fatalf("expected error %v, got %v", test.expectedError, err)
			}

			if !reflect.DeepEqual(args, test.expected) {
				t.Fatalf("expected args %v, got %v", test.expected, args)
			}
		})
	}
}

func TestToRecommendFollows(t *testing.T) {
	tests := []struct {
		name          string
		req           *Request
		expected      *RecommendFollowsArgs
		expectedError error
	}{
		{
			name:          "non-empty targets",
			req:           &Request{Targets: []string{pip}},
			expectedError: ErrParamNotSupported,
		},
		{
			name:          "search",
			req:           &Request{Search: "j"},
			expectedError: ErrParamNotSupported,
		},
		{
			name:          "invalid source key",
			req:           &Request{Algorithm: Algorithm{Sort: Personalized, Source: "abc"}, Limit: 10},
			expectedError: ErrInvalidSource,
		},
		{
			name:          "negative limit",
			req:           &Request{Algorithm: Algorithm{Sort: Global}, Limit: -1},
			expectedError: ErrInvalidLimit,
		},
		{
			name:          "limit too high",
			req:           &Request{Algorithm: Algorithm{Sort: Global}, Limit: StandardMaxLimit + 1},
			expectedError: ErrInvalidLimit,
		},
		{
			name: "valid",
			req: &Request{
				Algorithm: Algorithm{Sort: Personalized, Source: "npub176p7sup477k5738qhxx0hk2n0cty2k5je5uvalzvkvwmw4tltmeqw7vgup"},
				Limit:     69,
			},
			expected: &RecommendFollowsArgs{
				Algorithm: Algorithm{Sort: Personalized, Source: pip},
				Limit:     69,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args, err := test.req.ToRecommendFollowsArgs()
			if !errors.Is(err, test.expectedError) {
				t.Fatalf("expected error %v, got %v", test.expectedError, err)
			}

			if !reflect.DeepEqual(args, test.expected) {
				t.Fatalf("expected args %v, got %v", test.expected, args)
			}
		})
	}
}

func TestToRankProfiles(t *testing.T) {
	tests := []struct {
		name          string
		req           *Request
		expected      *RankProfilesArgs
		expectedError error
	}{
		{
			name:          "search",
			req:           &Request{Search: "j"},
			expectedError: ErrParamNotSupported,
		},
		{
			name:          "invalid source key",
			req:           &Request{Algorithm: Algorithm{Sort: Personalized, Source: "abc"}, Limit: 10, Targets: []string{pip}},
			expectedError: ErrInvalidSource,
		},
		{
			name:          "empty targets",
			req:           &Request{Algorithm: Algorithm{Sort: Global}, Limit: 10},
			expectedError: ErrInvalidTarget,
		},
		{
			name:          "too many targets",
			req:           &Request{Algorithm: Algorithm{Sort: Global}, Limit: 10, Targets: slice(ExtendedMaxLimit + 1)},
			expectedError: ErrInvalidTarget,
		},
		{
			name:          "negative limit",
			req:           &Request{Algorithm: Algorithm{Sort: Global}, Targets: []string{pip}, Limit: -1},
			expectedError: ErrInvalidLimit,
		},
		{
			name:          "limit too high",
			req:           &Request{Algorithm: Algorithm{Sort: Global}, Targets: []string{pip}, Limit: ExtendedMaxLimit + 1},
			expectedError: ErrInvalidLimit,
		},
		{
			name: "valid",
			req: &Request{
				Algorithm: Algorithm{Sort: Personalized, Source: "npub176p7sup477k5738qhxx0hk2n0cty2k5je5uvalzvkvwmw4tltmeqw7vgup"},
				Targets:   []string{odell, calle, pip, pip, "npub16kkn6vg4m8aqw5qtqmxdpwtqtkvg3gsx4jazpg0zu6q7c2gsjwrs3tdflr", "zzz"},
				Limit:     69,
			},
			expected: &RankProfilesArgs{
				Algorithm: Algorithm{Sort: Personalized, Source: pip},
				Targets:   []string{odell, calle, pip, randomKey, "zzz"},
				Limit:     5,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args, err := test.req.ToRankProfilesArgs()
			if !errors.Is(err, test.expectedError) {
				t.Fatalf("expected error %v, got %v", test.expectedError, err)
			}

			if !reflect.DeepEqual(args, test.expected) {
				t.Fatalf("expected args %v, got %v", test.expected, args)
			}
		})
	}
}

func TestToSearchProfiles(t *testing.T) {
	tests := []struct {
		name          string
		req           *Request
		expected      *SearchProfilesArgs
		expectedError error
	}{
		{
			name:          "non-empty targets",
			req:           &Request{Targets: []string{pip, calle}},
			expectedError: ErrParamNotSupported,
		},
		{
			name:          "invalid source key",
			req:           &Request{Algorithm: Algorithm{Sort: Personalized, Source: "abc"}, Limit: 10, Search: "jack"},
			expectedError: ErrInvalidSource,
		},
		{
			name:          "search too short",
			req:           &Request{Algorithm: Algorithm{Sort: Global}, Limit: 10, Search: "ab"},
			expectedError: ErrInvalidSearch,
		},
		{
			name:          "negative limit",
			req:           &Request{Algorithm: Algorithm{Sort: Global}, Limit: -1, Search: "jack"},
			expectedError: ErrInvalidLimit,
		},
		{
			name:          "limit too high",
			req:           &Request{Algorithm: Algorithm{Sort: Global}, Limit: StandardMaxLimit + 1, Search: "jack"},
			expectedError: ErrInvalidLimit,
		},
		{
			name: "valid",
			req: &Request{
				Algorithm: Algorithm{Sort: Personalized, Source: "npub176p7sup477k5738qhxx0hk2n0cty2k5je5uvalzvkvwmw4tltmeqw7vgup"},
				Search:    "   jack   ",
				Limit:     69,
			},
			expected: &SearchProfilesArgs{
				Algorithm: Algorithm{Sort: Personalized, Source: pip},
				Search:    "jack",
				Limit:     69,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args, err := test.req.ToSearchProfilesArgs()
			if !errors.Is(err, test.expectedError) {
				t.Fatalf("expected error %v, got %v", test.expectedError, err)
			}

			if !reflect.DeepEqual(args, test.expected) {
				t.Fatalf("expected args %v, got %v", test.expected, args)
			}
		})
	}
}

// ---------------------------------BENCHMARKS---------------------------------

func BenchmarkParse(b *testing.B) {
	candidates := nostr.Tags{
		{"param", "target", pip},
		{"param", "limit", "69"},
		{"param", "search", "jack"},
	}

	tags := make(nostr.Tags, ExtendedMaxLimit)
	for i := 0; i < ExtendedMaxLimit; i++ {
		index := rand.IntN(len(candidates))
		tags[i] = candidates[index]
	}

	req := &nostr.Event{Tags: tags}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		Parse(req)
	}
}

// ---------------------------------- HELPERS ---------------------------------

func slice(n int) []string {
	slice := make([]string, n)
	for i := range n {
		slice[i] = strconv.Itoa(i)
	}
	return slice
}
