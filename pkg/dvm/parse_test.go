package dvm

import (
	"errors"
	"reflect"
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

func TestParseArgs(t *testing.T) {
	testCases := []struct {
		name          string
		req           *nostr.Event
		expectedArgs  *Args
		expectedError error
	}{
		{
			name:          "nil req",
			req:           nil,
			expectedArgs:  nil,
			expectedError: ErrNilEvent,
		},
		{
			name: "empty req --> default args",
			req: &nostr.Event{
				PubKey: fran,
				Kind:   KindVerifyReputation,
			},
			expectedArgs: NewArgs("", fran, KindVerifyReputation),
		},
		{
			name: "invalid kind",
			req: &nostr.Event{
				ID:     "xxx",
				PubKey: fran,
				Kind:   43223,
			},
			expectedArgs:  NewArgs("xxx", fran, 43223),
			expectedError: ErrInvalidKind,
		},
		{
			name: "badly formatted tag: no param",
			req: &nostr.Event{
				PubKey: fran,
				Kind:   KindVerifyReputation,
				Tags: nostr.Tags{
					{"target", "xxxx"},
				},
			},
			expectedArgs:  NewArgs("", fran, KindVerifyReputation),
			expectedError: ErrBadlyFormattedTag,
		},
		{
			name: "badly formatted tag: too short",
			req: &nostr.Event{
				PubKey: fran,
				Kind:   KindVerifyReputation,
				Tags: nostr.Tags{
					{"param", "target"},
				},
			},
			expectedArgs:  NewArgs("", fran, KindVerifyReputation),
			expectedError: ErrBadlyFormattedTag,
		},
		{
			name: "invalid parameter",
			req: &nostr.Event{
				PubKey: fran,
				Kind:   KindVerifyReputation,
				Tags: nostr.Tags{
					{"param", "delta", "xxx"},
				},
			},

			expectedArgs:  NewArgs("", fran, KindVerifyReputation),
			expectedError: ErrUnknownParameter,
		},
		{
			name: "invalid sort option",
			req: &nostr.Event{
				PubKey: fran,
				Kind:   KindVerifyReputation,
				Tags: nostr.Tags{
					{"param", "sort", "grapeWine"},
				},
			},

			expectedArgs:  NewArgs("", fran, KindVerifyReputation),
			expectedError: ErrInvalidSortOption,
		},
		{
			name: "badly formatted pubkey",
			req: &nostr.Event{
				PubKey: fran,
				Kind:   KindVerifyReputation,
				Tags: nostr.Tags{
					{"param", "target", "xxxx"},
				},
			},
			expectedArgs:  NewArgs("", fran, KindVerifyReputation),
			expectedError: ErrBadlyFormattedKey,
		},
		{
			name: "badly formatted int",
			req: &nostr.Event{
				PubKey: fran,
				Kind:   KindVerifyReputation,
				Tags: nostr.Tags{
					{"param", "limit", "one"},
				},
			},
			expectedArgs:  NewArgs("", fran, KindVerifyReputation),
			expectedError: ErrBadlyFormattedInt,
		},
		{
			name: "limit too high",
			req: &nostr.Event{
				PubKey: fran,
				Kind:   KindVerifyReputation,
				Tags: nostr.Tags{
					{"param", "limit", "10000"},
				},
			},
			expectedArgs:  NewArgs("", fran, KindVerifyReputation),
			expectedError: ErrInvalidLimit,
		},
		{
			name: "valid relevant who follow",
			req: &nostr.Event{
				Kind:   KindVerifyReputation,
				PubKey: fran,
				Tags: nostr.Tags{
					{"param", "source", pip},
					{"param", "target", calle},
					{"param", "sort", "globalPagerank"},
				},
			},
			expectedArgs: &Args{
				Kind:    KindVerifyReputation,
				Pubkey:  fran,
				Source:  pip,
				Targets: []string{calle},
				Sort:    "globalPagerank",
				Limit:   DefaultLimit,
			},
		},
		{
			name: "valid recommended follows",
			req: &nostr.Event{
				Kind:   KindRecommendFollows,
				PubKey: pip,
				Tags: nostr.Tags{
					{"param", "sort", "personalizedPagerank"},
				},
			},
			expectedArgs: &Args{
				Kind:    KindRecommendFollows,
				Pubkey:  pip,
				Source:  pip,
				Targets: nil,
				Sort:    "personalizedPagerank",
				Limit:   DefaultLimit,
			},
		},
		{
			name: "valid sort authors",
			req: &nostr.Event{
				Kind:   KindSortProfiles,
				PubKey: pip,
				Tags: nostr.Tags{
					{"param", "target", fran},
					{"param", "target", pip},
					{"param", "target", calle},
					{"param", "target", "npub1glq5d270lwhzp9eqtw5t6f204f0hcgcgedlclhe0kcqk7jccw4wscjh0u8"},
				},
			},
			expectedArgs: &Args{
				Kind:    KindSortProfiles,
				Pubkey:  pip,
				Source:  pip,
				Targets: []string{fran, pip, calle, "47c146abcffbae2097205ba8bd254faa5f7c2308cb7f8fdf2fb6016f4b18755d"},
				Sort:    "globalPagerank",
				Limit:   DefaultLimit,
			},
		},
		{
			name: "valid search authors",
			req: &nostr.Event{
				Kind:   KindSearchProfiles,
				PubKey: pip,
				Tags: nostr.Tags{
					{"param", "search", "hello"},
					{"param", "sort", "personalizedPagerank"},
				},
			},
			expectedArgs: &Args{
				Kind:    KindSearchProfiles,
				Pubkey:  pip,
				Source:  pip,
				Targets: nil,
				Sort:    "personalizedPagerank",
				Limit:   DefaultLimit,
				Search:  "hello",
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			args, err := Parse(test.req)

			if !errors.Is(err, test.expectedError) {
				t.Fatalf("ParseRequestArgs(): expected %v, got %v", test.expectedError, err)
			}

			if !reflect.DeepEqual(args, test.expectedArgs) {
				t.Errorf("ParseRequestArgs(): expected %v, got %v", test.expectedArgs, args)
			}
		})
	}
}

// ----------------------------------BENCHMARKS--------------------------------

func BenchmarkParseArgs(b *testing.B) {
	const npub = "npub1wf4pufsucer5va8g9p0rj5dnhvfeh6d8w0g6eayaep5dhps6rsgs43dgh9"
	const tagsNum = 10000

	tags := make([]nostr.Tag, tagsNum)
	for i := 0; i < tagsNum; i++ {
		tags[i] = nostr.Tag{"param", "target", npub}
	}

	req := &nostr.Event{
		PubKey: fran,
		Tags:   tags,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Parse(req)
	}
}
