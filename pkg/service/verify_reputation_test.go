package service

import (
	"errors"
	"reflect"
	"testing"
)

// pubkeys for testing purposes
const (
	fran  string = "726a1e261cc6474674e8285e3951b3bb139be9a773d1acf49dc868db861a1c11"
	odell string = "04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9"
	calle string = "50d94fc2d8580c682b071a542f8b1e31a200b0508bab95a33bef0855df281d63"

	pip     string = "f683e87035f7ad4f44e0b98cfbd9537e16455a92cd38cefc4cb31db7557f5ef2"
	pipNpub string = "npub176p7sup477k5738qhxx0hk2n0cty2k5je5uvalzvkvwmw4tltmeqw7vgup"
)

func TestVerifyReputationNormalize(t *testing.T) {
	tests := []struct {
		name     string
		args     *VerifyReputationArgs
		expected *VerifyReputationArgs
		err      error
	}{
		{
			name: "invalid limit (negative)",
			args: &VerifyReputationArgs{Algorithm: Algorithm{Sort: Global}, Target: pip, Limit: -1},
			err:  ErrInvalidLimit,
		},
		{
			name: "invalid limit (too high)",
			args: &VerifyReputationArgs{Algorithm: Algorithm{Sort: Global}, Target: pip, Limit: 101},
			err:  ErrInvalidLimit,
		},
		{
			name: "invalid sort",
			args: &VerifyReputationArgs{Algorithm: Algorithm{Sort: "unknown"}, Limit: 5},
			err:  ErrInvalidSort,
		},
		{
			name: "missing source",
			args: &VerifyReputationArgs{Algorithm: Algorithm{Sort: Personalized}, Limit: 5},
			err:  ErrInvalidSource,
		},
		{
			name: "invalid source",
			args: &VerifyReputationArgs{Algorithm: Algorithm{Sort: Personalized, Source: "abc"}, Limit: 5},
			err:  ErrInvalidSource,
		},
		{
			name: "invalid target",
			args: &VerifyReputationArgs{Algorithm: Algorithm{Sort: Global}, Target: "xxx", Limit: 10},
			err:  ErrInvalidTarget,
		},
		{
			name:     "valid (target is npub)",
			args:     &VerifyReputationArgs{Algorithm: Algorithm{Sort: Global}, Target: pipNpub, Limit: 10},
			expected: &VerifyReputationArgs{Algorithm: Algorithm{Sort: Global}, Target: pip, Limit: 10},
		},
		{
			name:     "valid (source is npub)",
			args:     &VerifyReputationArgs{Algorithm: Algorithm{Sort: Personalized, Source: pipNpub}, Target: fran, Limit: 10},
			expected: &VerifyReputationArgs{Algorithm: Algorithm{Sort: Personalized, Source: pip}, Target: fran, Limit: 10},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.args.Normalize()
			if !errors.Is(err, test.err) {
				t.Fatalf("expected error %v, got %v", test.err, err)
			}

			if err == nil && !reflect.DeepEqual(test.args, test.expected) {
				t.Fatalf("expected args %v, got %v", test.expected, test.args)
			}
		})
	}
}
