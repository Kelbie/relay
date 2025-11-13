package service

import (
	"errors"
	"math/rand/v2"
	"reflect"
	"testing"
)

func TestRankProfilesNormalize(t *testing.T) {
	tests := []struct {
		name     string
		args     *RankProfilesArgs
		expected *RankProfilesArgs
		err      error
	}{
		{
			name: "invalid limit (negative)",
			args: &RankProfilesArgs{Algorithm: Algorithm{Sort: Global}, Targets: []string{pip}, Limit: -1},
			err:  ErrInvalidLimit,
		},
		{
			name: "invalid limit (too high)",
			args: &RankProfilesArgs{Algorithm: Algorithm{Sort: Global}, Targets: []string{pip}, Limit: 1001},
			err:  ErrInvalidLimit,
		},
		{
			name: "invalid sort",
			args: &RankProfilesArgs{Algorithm: Algorithm{Sort: "unknown"}, Limit: 5},
			err:  ErrInvalidSort,
		},
		{
			name: "missing source",
			args: &RankProfilesArgs{Algorithm: Algorithm{Sort: Personalized}, Limit: 5},
			err:  ErrInvalidSource,
		},
		{
			name: "invalid source",
			args: &RankProfilesArgs{Algorithm: Algorithm{Sort: Personalized, Source: "abc"}, Limit: 5},
			err:  ErrInvalidSource,
		},
		{
			name: "no targets",
			args: &RankProfilesArgs{Algorithm: Algorithm{Sort: Global}, Limit: 10},
			err:  ErrInvalidTargets,
		},
		{
			name: "too many targets",
			args: &RankProfilesArgs{Algorithm: Algorithm{Sort: Global}, Targets: randomTargets(1001), Limit: 10},
			err:  ErrInvalidTargets,
		},
		{
			name:     "valid (target is npub)",
			args:     &RankProfilesArgs{Algorithm: Algorithm{Sort: Global}, Targets: []string{pipNpub, pipNpub, "aaa"}, Limit: 10},
			expected: &RankProfilesArgs{Algorithm: Algorithm{Sort: Global}, Targets: []string{pip, "aaa"}, Limit: 2},
		},
		{
			name:     "valid (source is npub)",
			args:     &RankProfilesArgs{Algorithm: Algorithm{Sort: Personalized, Source: pipNpub}, Targets: []string{"aaa"}, Limit: 10},
			expected: &RankProfilesArgs{Algorithm: Algorithm{Sort: Personalized, Source: pip}, Targets: []string{"aaa"}, Limit: 1},
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

func randomTargets(n int) []string {
	s := make([]string, n)
	for i := range n {
		s[i] = randomString(10)
	}
	return s
}

const symbols = "abcdefghijklmnopqrstuvwxyz"

func randomString(l int) string {
	s := make([]byte, l)
	for range l {
		idx := rand.IntN(len(symbols))
		s = append(s, symbols[idx])
	}
	return string(s)
}
