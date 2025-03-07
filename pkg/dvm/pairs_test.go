package dvm

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"sort"
	"testing"

	"cmp"
)

func TestTop(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		tests := []struct {
			name     string
			limit    int
			pairs    Pairs[int, float64]
			expected Pairs[int, float64]
		}{
			{
				name:  "nil pairs",
				limit: 1,
			},
			{
				name:  "empty pairs",
				limit: 1,
				pairs: Pairs[int, float64]{},
			},
			{
				name:  "limit = 0",
				limit: 0,
				pairs: Pairs[int, float64]{{Key: 0, Rank: 1}},
			},
			{
				name:     "limit bigger than the lenght of the pairs",
				limit:    10,
				pairs:    Pairs[int, float64]{{Key: 0, Rank: 1}},
				expected: Pairs[int, float64]{{Key: 0, Rank: 1}},
			},
			{
				name:  "valid",
				limit: 3,
				pairs: Pairs[int, float64]{
					{Key: 0, Rank: 1},
					{Key: 1, Rank: 1.5},
					{Key: 2, Rank: -0.2},
					{Key: 3, Rank: 0.0},
					{Key: 4, Rank: 11.0},
					{Key: 5, Rank: 6.1},
				},
				expected: Pairs[int, float64]{
					{Key: 4, Rank: 11.0},
					{Key: 5, Rank: 6.1},
					{Key: 1, Rank: 1.5},
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				pairs := Top(test.pairs, test.limit)
				if !reflect.DeepEqual(pairs, test.expected) {
					t.Fatalf("Top: expected pairs %v, got %v", test.expected, pairs)
				}
			})
		}
	})

	t.Run("fuzzy", func(t *testing.T) {
		const iter = 1000
		const maxSize = 1000

		for i := 0; i < iter; i++ {
			size := rand.IntN(maxSize)
			limit := rand.IntN(maxSize)
			pairs := randPairs(size)

			top := Top(pairs, limit)
			expected := naiveTop(pairs, limit)

			if !reflect.DeepEqual(top, expected) {
				t.Errorf("len(pairs) = %d; limit = %d", len(pairs), limit)
				t.Fatalf("got %v, expected %v", top, expected)
			}
		}
	})
}

// ------------------------------------BENCHMARK--------------------------------

func BenchmarkTop(b *testing.B) {
	sizes := []int{10000, 100000, 1000000}
	limits := []int{10, 100, 1000}

	for _, size := range sizes {
		for _, limit := range limits {
			b.Run(fmt.Sprintf("top %d/%d", limit, size), func(b *testing.B) {

				pairs := randPairs(size)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					Top(pairs, limit)
				}
			})
		}
	}
}

func BenchmarkNaiveTop(b *testing.B) {
	sizes := []int{10000, 100000, 1000000}
	limits := []int{10, 100, 1000}

	for _, size := range sizes {
		for _, limit := range limits {
			b.Run(fmt.Sprintf("naive top %d/%d", limit, size), func(b *testing.B) {

				pairs := randPairs(size)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					naiveTop(pairs, limit)
				}
			})
		}
	}
}

// ------------------------------------HELPERS----------------------------------

func randPairs(size int) Pairs[int, float64] {
	pairs := make(Pairs[int, float64], size)
	for j := 0; j < size; j++ {
		pairs[j] = Pair[int, float64]{Key: j, Rank: rand.Float64()}
	}

	return pairs
}

// naiveTop() is only used to test topPairs by comparing their results.
func naiveTop[K comparable, V cmp.Ordered](pairs Pairs[K, V], limit int) Pairs[K, V] {
	if limit < 1 || len(pairs) < 1 {
		return nil
	}

	sort.Sort(pairs)
	if limit >= len(pairs) {
		return pairs
	}

	return pairs[:limit]
}
