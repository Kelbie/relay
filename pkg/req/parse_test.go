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
	record := dvm.Record{Kind: dvm.KindVerifyReputation, Timestamp: nostr.Now()}
	tests := []struct {
		name          string
		filter        *nostr.Filter
		expected      *dvm.Request
		expectedError error
	}{
		{
			name:          "empty search",
			filter:        &nostr.Filter{Search: ""},
			expectedError: ErrEmptyFieldSearch,
		},
		{
			name: "valid 1",
			filter: &nostr.Filter{
				Kinds: []int{dvm.KindVerifyReputation + 1000, dvm.KindDVMError},
				Search: `{
					"source": "726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11",
					"targets": ["04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9", "f683e87035f7ad4f44e0b98cfbd9537e16455a92cd38cefc4cb31db7557f5ef2"],
					"limit": 100,
					"search": "jack"
				}`},
			expected: &dvm.Request{
				Record:    record,
				Algorithm: dvm.Algorithm{Sort: dvm.Global, Source: fran},
				Targets:   []string{odell, pip},
				Limit:     100,
				Search:    "jack",
			},
		},
		{
			name: "valid 2",
			filter: &nostr.Filter{
				Kinds: []int{dvm.KindVerifyReputation + 1000, dvm.KindDVMError},
				Search: `{
					"source": "726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11",
					"targets": ["04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9", "f683e87035f7ad4f44e0b98cfbd9537e16455a92cd38cefc4cb31db7557f5ef2"],
					"limit": 100,
					"search": "jack"
				}`},
			expected: &dvm.Request{
				Record:    record,
				Algorithm: dvm.Algorithm{Sort: dvm.Global, Source: fran},
				Targets:   []string{odell, pip},
				Limit:     100,
				Search:    "jack",
			},
		},
		{
			name: "valid 3",
			filter: &nostr.Filter{
				Kinds:  []int{dvm.KindVerifyReputation + 1000, dvm.KindDVMError},
				Search: "{\"source\":\"04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9\", \"targets\":[\"726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11\"]}",
			},
			expected: &dvm.Request{
				Record:    record,
				Algorithm: dvm.Algorithm{Sort: dvm.Global, Source: odell},
				Targets:   []string{fran},
				Limit:     dvm.DefaultLimit,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args, err := Parse(test.filter)

			if !errors.Is(err, test.expectedError) {
				t.Fatalf("Parse: expected error %v, got %v", test.expectedError, err)
			}

			if !reflect.DeepEqual(args, test.expected) {
				t.Errorf("Parse: expected request %v, got %v", test.expected, args)
			}
		})
	}
}

func TestValidateFilter(t *testing.T) {
	tests := []struct {
		name          string
		filter        *nostr.Filter
		expectedError error
	}{
		{
			name:          "invalid kinds 1",
			filter:        &nostr.Filter{Kinds: []int{69}, Search: "xx"},
			expectedError: ErrInvalidKindsFormat,
		},
		{
			name:          "invalid kinds 2",
			filter:        &nostr.Filter{Kinds: []int{6312, 6313}, Search: "xx"},
			expectedError: ErrInvalidKindsFormat,
		},
		{
			name:          "invalid kinds 3",
			filter:        &nostr.Filter{Kinds: []int{6312, 7000, 1}, Search: "xx"},
			expectedError: ErrInvalidKindsFormat,
		},
		{
			name:   "valid",
			filter: &nostr.Filter{Kinds: []int{6312, 7000}, Search: "xx"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			err := ValidateFilter(test.filter)
			if !errors.Is(err, test.expectedError) {
				t.Fatalf("expected error %v, got %v", test.expectedError, err)
			}
		})
	}
}
