package dvm

import (
	"errors"
	"reflect"
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

// pubkeys for testing purposes
var fran string = "726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11"
var pk1 string = "d05ab982e1105476ab68e4c6728d148f8e6222154e60cc359ef6b8599c820bea"
var pk2 string = "6efd1b46b3e6d1ec2447af7c905827bc83e1330bee2c3a6a5b8e0769734785e2"

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
			expectedError: nil,
		},
		{
			name: "empty req --> default args",
			req: &nostr.Event{
				PubKey: fran,
			},
			expectedArgs:  NewArgs(fran),
			expectedError: nil,
		},
		{
			name: "invalid sort option",
			req: &nostr.Event{
				PubKey: fran,
				Tags: nostr.Tags{
					{"param", "sort", "grapeWine"},
				},
			},

			expectedArgs:  nil,
			expectedError: ErrInvalidSortOption,
		},
		{
			name: "badly formatted tag: no param",
			req: &nostr.Event{
				PubKey: fran,
				Tags: nostr.Tags{
					{"target", "xxxx"},
				},
			},
			expectedArgs:  nil,
			expectedError: ErrBadlyFormattedTag,
		},
		{
			name: "badly formatted tag: too short",
			req: &nostr.Event{
				PubKey: fran,
				Tags: nostr.Tags{
					{"param", "target"},
				},
			},
			expectedArgs:  nil,
			expectedError: ErrBadlyFormattedTag,
		},
		{
			name: "badly formatted key",
			req: &nostr.Event{
				PubKey: fran,
				Tags: nostr.Tags{
					{"param", "target", "xxxx"},
				},
			},
			expectedArgs:  nil,
			expectedError: ErrBadlyFormattedKey,
		},
		{
			name: "badly formatted int",
			req: &nostr.Event{
				PubKey: fran,
				Tags: nostr.Tags{
					{"param", "distance", "one"},
				},
			},
			expectedArgs:  nil,
			expectedError: ErrBadlyFormattedInt,
		},
		{
			name: "valid relevant who follow",
			req: &nostr.Event{
				PubKey: fran,
				Tags: nostr.Tags{
					{"param", "source", pk1},
					{"param", "target", pk2},
					{"param", "sort", "global"},
				},
			},
			expectedArgs: &Args{
				Source:   pk1,
				Targets:  []string{pk2},
				Sort:     "global",
				Distance: defaultDistance,
				Limit:    defaultLimit,
			},
			expectedError: nil,
		},
		{
			name: "valid recommended follows",
			req: &nostr.Event{
				PubKey: pk1,
				Tags: nostr.Tags{
					{"param", "sort", "personalized"},
				},
			},
			expectedArgs: &Args{
				Source:   pk1,
				Targets:  []string{},
				Sort:     "personalized",
				Distance: defaultDistance,
				Limit:    defaultLimit,
			},
			expectedError: nil,
		},
		{
			name: "valid sort authors",
			req: &nostr.Event{
				PubKey: pk1,
				Tags: nostr.Tags{
					{"param", "target", fran},
					{"param", "target", pk1},
					{"param", "target", pk2},
				},
			},
			expectedArgs: &Args{
				Source:   pk1,
				Targets:  []string{fran, pk1, pk2},
				Sort:     "global",
				Distance: defaultDistance,
				Limit:    defaultLimit,
			},
			expectedError: nil,
		},
		{
			name: "valid impersonator detection",
			req: &nostr.Event{
				PubKey: pk1,
				Tags: nostr.Tags{
					{"param", "target", fran},
				},
			},
			expectedArgs: &Args{
				Source:   pk1,
				Targets:  []string{fran},
				Sort:     "global",
				Distance: defaultDistance,
				Limit:    defaultLimit,
			},
			expectedError: nil,
		},
		{
			name: "valid impersonator detection",
			req: &nostr.Event{
				PubKey: pk1,
				Tags: nostr.Tags{
					{"param", "target", "npub1glq5d270lwhzp9eqtw5t6f204f0hcgcgedlclhe0kcqk7jccw4wscjh0u8"},
				},
			},
			expectedArgs: &Args{
				Source:   pk1,
				Targets:  []string{"47c146abcffbae2097205ba8bd254faa5f7c2308cb7f8fdf2fb6016f4b18755d"},
				Sort:     "global",
				Distance: defaultDistance,
				Limit:    defaultLimit,
			},
			expectedError: nil,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			args, err := ParseArgs(test.req)

			if !errors.Is(err, test.expectedError) {
				t.Fatalf("ParseRequestArgs(): expected %v, got %v", test.expectedError, err)
			}

			if !reflect.DeepEqual(args, test.expectedArgs) {
				t.Errorf("ParseRequestArgs(): expected %v, got %v", test.expectedArgs, args)
			}
		})
	}
}
