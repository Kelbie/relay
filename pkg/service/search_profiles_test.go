package service

import (
	"errors"
	"reflect"
	"testing"
)

func TestSearchProfilesNormalize(t *testing.T) {
	tests := []struct {
		name     string
		args     *SearchProfilesArgs
		expected *SearchProfilesArgs
		err      error
	}{
		{
			name: "invalid limit (negative)",
			args: &SearchProfilesArgs{Algorithm: Algorithm{Sort: Global}, Search: "jack", Limit: -1},
			err:  ErrInvalidLimit,
		},
		{
			name: "invalid limit (too high)",
			args: &SearchProfilesArgs{Algorithm: Algorithm{Sort: Global}, Search: "jack", Limit: 101},
			err:  ErrInvalidLimit,
		},
		{
			name: "invalid sort",
			args: &SearchProfilesArgs{Algorithm: Algorithm{Sort: "unknown"}, Search: "jack", Limit: 5},
			err:  ErrInvalidSort,
		},
		{
			name: "missing source",
			args: &SearchProfilesArgs{Algorithm: Algorithm{Sort: Personalized}, Search: "jack", Limit: 5},
			err:  ErrInvalidSource,
		},
		{
			name: "invalid source",
			args: &SearchProfilesArgs{Algorithm: Algorithm{Sort: Personalized, Source: "abc"}, Search: "jack", Limit: 5},
			err:  ErrInvalidSource,
		},
		{
			name: "invalid search (too short)",
			args: &SearchProfilesArgs{Algorithm: Algorithm{Sort: Global}, Search: "ja", Limit: 10},
			err:  ErrInvalidSearch,
		},
		{
			name: "invalid search (too long)",
			args: &SearchProfilesArgs{Algorithm: Algorithm{Sort: Global}, Search: randomString(101), Limit: 10},
			err:  ErrInvalidSearch,
		},
		{
			name:     "valid (source is npub)",
			args:     &SearchProfilesArgs{Algorithm: Algorithm{Sort: Personalized, Source: pipNpub}, Search: "jack", Limit: 10},
			expected: &SearchProfilesArgs{Algorithm: Algorithm{Sort: Personalized, Source: pip}, Search: "jack", Limit: 10},
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
