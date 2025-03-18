package pairs

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"reflect"
	"sort"
	"testing"
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
				pairs: Pairs[int, float64]{{Key: 0, Val: 1}},
			},
			{
				name:     "limit bigger than the lenght of the pairs",
				limit:    10,
				pairs:    Pairs[int, float64]{{Key: 0, Val: 1}},
				expected: Pairs[int, float64]{{Key: 0, Val: 1}},
			},
			{
				name:  "valid",
				limit: 3,
				pairs: Pairs[int, float64]{
					{Key: 0, Val: 1},
					{Key: 1, Val: 1.5},
					{Key: 2, Val: -0.2},
					{Key: 3, Val: 0.0},
					{Key: 4, Val: 11.0},
					{Key: 5, Val: 6.1},
				},
				expected: Pairs[int, float64]{
					{Key: 4, Val: 11.0},
					{Key: 5, Val: 6.1},
					{Key: 1, Val: 1.5},
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				top := test.pairs.Top(test.limit)
				if !reflect.DeepEqual(top, test.expected) {
					t.Fatalf("Top: expected pairs %v, got %v", test.expected, top)
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

			top := pairs.Top(limit)
			expected := pairs.naiveTop(limit)

			if !reflect.DeepEqual(top, expected) {
				t.Errorf("len(pairs) = %d; limit = %d", len(pairs), limit)
				t.Fatalf("got %v, expected %v", top, expected)
			}
		}
	})
}

func TestTopMap(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		tests := []struct {
			name     string
			limit    int
			m        map[int]float64
			expected Pairs[int, float64]
		}{
			{
				name:  "nil map",
				limit: 1,
			},
			{
				name:  "empty map",
				limit: 1,
				m:     map[int]float64{},
			},
			{
				name:  "limit = 0",
				limit: 0,
				m:     map[int]float64{0: 1},
			},
			{
				name:     "limit bigger than the lenght of the map",
				limit:    10,
				m:        map[int]float64{0: 1},
				expected: Pairs[int, float64]{{Key: 0, Val: 1}},
			},
			{
				name:  "valid",
				limit: 3,
				m:     map[int]float64{0: 1, 1: 1.5, 2: -0.2, 3: 0, 4: 11, 5: 6.1},
				expected: Pairs[int, float64]{
					{Key: 4, Val: 11.0},
					{Key: 5, Val: 6.1},
					{Key: 1, Val: 1.5},
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				top := Top(test.m, test.limit)
				if !reflect.DeepEqual(top, test.expected) {
					t.Fatalf("Top: expected pairs %v, got %v", test.expected, top)
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
			m := randMap(size)

			top := Top(m, limit)
			expected := naiveTop(m, limit)

			if !reflect.DeepEqual(top, expected) {
				t.Errorf("len(map) = %d; limit = %d", len(m), limit)
				t.Fatalf("got %v, expected %v", top, expected)
			}
		}
	})
}

// ------------------------------------BENCHMARK--------------------------------

func BenchmarkMin(b *testing.B) {
	sizes := []int{10000, 100000, 1000000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {

			pairs := randPairs(size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				pairs.Min()
			}
		})
	}
}

func BenchmarkTop(b *testing.B) {
	sizes := []int{10000, 100000, 1000000}
	limits := []int{10, 100, 1000}

	for _, size := range sizes {
		for _, limit := range limits {
			b.Run(fmt.Sprintf("top %d/%d", limit, size), func(b *testing.B) {

				pairs := randPairs(size)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					pairs.Top(limit)
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
					pairs.naiveTop(limit)
				}
			})
		}
	}
}

func BenchmarkTopMap(b *testing.B) {
	sizes := []int{10000, 100000, 1000000}
	limits := []int{10, 100, 1000}

	for _, size := range sizes {
		for _, limit := range limits {
			b.Run(fmt.Sprintf("top %d/%d", limit, size), func(b *testing.B) {

				m := randMap(size)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					Top(m, limit)
				}
			})
		}
	}
}

func BenchmarkNaiveTopMap(b *testing.B) {
	sizes := []int{10000, 100000, 1000000}
	limits := []int{10, 100, 1000}

	for _, size := range sizes {
		for _, limit := range limits {
			b.Run(fmt.Sprintf("naive top %d/%d", limit, size), func(b *testing.B) {

				m := randMap(size)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					naiveTop(m, limit)
				}
			})
		}
	}
}

func BenchmarkToPairs(b *testing.B) {
	sizes := []int{10000, 100000, 1000000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("ToPairs size=%d", size), func(b *testing.B) {

			m := randMap(size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ToPairs(m)
			}
		})
	}
}

// ------------------------------------HELPERS----------------------------------

func randPairs(size int) Pairs[int, float64] {
	pairs := make(Pairs[int, float64], size)
	for j := 0; j < size; j++ {
		pairs[j] = Pair[int, float64]{Key: j, Val: rand.Float64()}
	}

	return pairs
}

func randMap(size int) map[int]float64 {
	m := make(map[int]float64, size)
	for j := 0; j < size; j++ {
		m[j] = rand.Float64()
	}

	return m
}

// naiveTop() is only used to test topPairs by comparing their results.
func (p Pairs[K, V]) naiveTop(limit int) Pairs[K, V] {
	if limit < 1 || len(p) < 1 {
		return nil
	}

	sort.Sort(p)
	if limit >= len(p) {
		return p
	}

	return p[:limit]
}

// naiveTop() is only used to test topPairs by comparing their results.
func naiveTop[K comparable, V cmp.Ordered](m map[K]V, limit int) Pairs[K, V] {
	if limit < 1 || len(m) < 1 {
		return nil
	}

	pairs := ToPairs(m)
	sort.Sort(pairs)

	if limit >= len(pairs) {
		return pairs
	}

	return pairs[:limit]
}
