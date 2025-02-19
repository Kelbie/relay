package req

import (
	"errors"
	"reflect"
	"testing"

	"github.com/vertex-lab/relay/pkg/dvm"

	"github.com/nbd-wtf/go-nostr"
)

// pubkeys for testing purposes
const (
	fran  string = "726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11"
	odell string = "04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9"
	calle string = "50d94fc2d8580c682b071a542f8b1e31a200b0508bab95a33bef0855df281d63"
	pip   string = "f683e87035f7ad4f44e0b98cfbd9537e16455a92cd38cefc4cb31db7557f5ef2"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		name          string
		filter        *nostr.Filter
		expectedArgs  *dvm.Args
		expectedError error
	}{
		{
			name:          "nil filter",
			filter:        nil,
			expectedArgs:  nil,
			expectedError: ErrNilFilter,
		},
		{
			name:          "empty search",
			filter:        &nostr.Filter{Search: ""},
			expectedArgs:  nil,
			expectedError: ErrEmptyFieldSearch,
		},
		{
			name:          "invalid kinds 1",
			filter:        &nostr.Filter{Kinds: []int{69}, Search: "xx"},
			expectedArgs:  nil,
			expectedError: ErrInvalidKindsFormat,
		},
		{
			name:          "invalid kinds 2",
			filter:        &nostr.Filter{Kinds: []int{6312, 6313}, Search: "xx"},
			expectedArgs:  nil,
			expectedError: ErrInvalidKindsFormat,
		},
		{
			name:          "invalid kinds 3",
			filter:        &nostr.Filter{Kinds: []int{6312, 7000, 1}, Search: "xx"},
			expectedArgs:  nil,
			expectedError: ErrInvalidKindsFormat,
		},
		{
			name: "invalid source",
			filter: &nostr.Filter{
				Kinds: []int{dvm.KindVerifyReputation + 1000, dvm.KindDVMError},
				Search: `{
					"source": "abc"
				}`},
			expectedArgs:  dvm.NewArgs("", "", dvm.KindVerifyReputation),
			expectedError: dvm.ErrBadlyFormattedKey,
		},
		{
			name: "invalid targets",
			filter: &nostr.Filter{
				Kinds: []int{dvm.KindVerifyReputation + 1000, dvm.KindDVMError},
				Search: `{
					"source": "726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11",
					"targets": ["abc", "cde"]
				}`},
			expectedArgs:  dvm.NewArgs("", "", dvm.KindVerifyReputation),
			expectedError: dvm.ErrBadlyFormattedKey,
		},
		{
			name: "invalid sort",
			filter: &nostr.Filter{
				Kinds: []int{dvm.KindVerifyReputation + 1000, dvm.KindDVMError},
				Search: `{
					"source": "726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11",
					"targets": ["04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9"],
					"sort": "abc"
				}`},
			expectedArgs:  dvm.NewArgs("", "", dvm.KindVerifyReputation),
			expectedError: dvm.ErrInvalidSortOption,
		},
		{
			name: "invalid limit",
			filter: &nostr.Filter{
				Kinds: []int{dvm.KindVerifyReputation + 1000, dvm.KindDVMError},
				Search: `{
					"source": "726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11",
					"targets": ["04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9"],
					"limit": 100000
				}`},
			expectedArgs:  dvm.NewArgs("", "", dvm.KindVerifyReputation),
			expectedError: dvm.ErrInvalidLimit,
		},
		{
			name: "valid",
			filter: &nostr.Filter{
				Kinds: []int{dvm.KindVerifyReputation + 1000, dvm.KindDVMError},
				Search: `{
					"source": "726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11",
					"targets": ["04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9", "f683e87035f7ad4f44e0b98cfbd9537e16455a92cd38cefc4cb31db7557f5ef2"],
					"limit": 100,
					"search":"jack",
				}`},
			expectedArgs: &dvm.Args{
				Kind:    dvm.KindVerifyReputation,
				Source:  fran,
				Targets: []string{odell, pip},
				Sort:    dvm.DefaultSort,
				Limit:   100,
				Search:  "jack",
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			args, err := Parse(test.filter)

			if !errors.Is(err, test.expectedError) {
				t.Fatalf("Parse: expected error %v, got %v", test.expectedError, err)
			}

			if !reflect.DeepEqual(args, test.expectedArgs) {
				t.Errorf("Parse: expected args %v, got %v", test.expectedArgs, args)
			}
		})
	}
}
