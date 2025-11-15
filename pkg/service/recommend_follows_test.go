package service

import (
	"errors"
	"reflect"
	"testing"
)

func TestRecommendFollowsNormalize(t *testing.T) {
	tests := []struct {
		name     string
		args     *RecommendFollowsArgs
		expected *RecommendFollowsArgs
		err      error
	}{
		{
			name: "invalid limit (negative)",
			args: &RecommendFollowsArgs{Algorithm: Algorithm{Sort: Global}, Limit: -1},
			err:  ErrInvalidLimit,
		},
		{
			name: "invalid limit (too high)",
			args: &RecommendFollowsArgs{Algorithm: Algorithm{Sort: Global}, Limit: 101},
			err:  ErrInvalidLimit,
		},
		{
			name: "invalid sort",
			args: &RecommendFollowsArgs{Algorithm: Algorithm{Sort: "unknown"}, Limit: 5},
			err:  ErrInvalidSort,
		},
		{
			name: "missing source",
			args: &RecommendFollowsArgs{Algorithm: Algorithm{Sort: Personalized}, Limit: 5},
			err:  ErrInvalidSource,
		},
		{
			name: "invalid source",
			args: &RecommendFollowsArgs{Algorithm: Algorithm{Sort: Personalized, Source: "abc"}, Limit: 5},
			err:  ErrInvalidSource,
		},
		{
			name:     "valid (source is npub)",
			args:     &RecommendFollowsArgs{Algorithm: Algorithm{Sort: Personalized, Source: pipNpub}, Limit: 10},
			expected: &RecommendFollowsArgs{Algorithm: Algorithm{Sort: Personalized, Source: pip}, Limit: 10},
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

func TestRecommendFollowsInterface(t *testing.T) {
	var _ Args = &RecommendFollowsArgs{}
}
