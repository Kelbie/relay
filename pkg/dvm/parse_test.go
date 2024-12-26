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
	type test struct {
		name          string
		req           *nostr.Event
		expectedArgs  *Args
		expectedError error
	}

	t.Run("simple errors", func(t *testing.T) {
		testCases := []test{
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
	})

	t.Run("valid", func(t *testing.T) {
		testCases := []test{
			{
				name:          "relevant who follow",
				req:           nil,
				expectedArgs:  nil,
				expectedError: nil,
			},
			{
				name:          "recommended follows",
				req:           nil,
				expectedArgs:  nil,
				expectedError: nil,
			},
			{
				name:          "sort authors",
				req:           nil,
				expectedArgs:  nil,
				expectedError: nil,
			},
			{
				name:          "impersonator detection",
				req:           nil,
				expectedArgs:  nil,
				expectedError: nil,
			},
			{
				name:          "degrees of separation",
				req:           nil,
				expectedArgs:  nil,
				expectedError: nil,
			},
			{
				name:          "verified followers count",
				req:           nil,
				expectedArgs:  nil,
				expectedError: nil,
			},
			{
				name:          "verified followers",
				req:           nil,
				expectedArgs:  nil,
				expectedError: nil,
			},
		}

		_ = testCases
	})

	// req := nostr.Event{
	// 	PubKey: fran,
	// 	Kind:   KindSortAuthors,
	// 	Tags: nostr.Tags{
	// 		{"param", "target", pk1},
	// 		{"param", "target", pk2},
	// 		{"param", "sort", "personalized"},
	// 	},
	// }

	// expectedArgs := &Args{
	// 	Source:   "",
	// 	Targets:  []string{pk1, pk2},
	// 	Distance: defaultDistance,
	// 	Sort:     "personalized",
	// 	Limit:    defaultLimit,
	// }
	// args, err := ParseArgs(&req)

	// if err != nil {
	// 	t.Fatalf("ParseRequestArgs(): expected %v, got %v", err, nil)
	// }

}
