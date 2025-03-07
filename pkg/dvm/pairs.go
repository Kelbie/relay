package dvm

import (
	"cmp"
	"fmt"
	"sort"
)

type Pair[K comparable, V cmp.Ordered] struct {
	Key  K `json:"pubkey"`
	Rank V `json:"rank"`
}

type Pairs[K comparable, V cmp.Ordered] []Pair[K, V]

func (p Pairs[K, V]) Len() int           { return len(p) }
func (p Pairs[K, V]) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p Pairs[K, V]) Less(i, j int) bool { return p[i].Rank > p[j].Rank }

// Min() returns the position and value of the smallest pair by Rank.
func (p Pairs[K, V]) Min() (int, V) {
	if len(p) < 1 {
		panic("dvm.Min: Pairs is empty")
	}

	index, min := 0, p[0].Rank
	for i, pair := range p {
		if pair.Rank < min {
			index = i
			min = pair.Rank
		}
	}
	return index, min
}

func (p Pairs[K, V]) Unpack() ([]K, []V) {
	keys := make([]K, len(p))
	vals := make([]V, len(p))

	for i, pair := range p {
		keys[i] = pair.Key
		vals[i] = pair.Rank
	}

	return keys, vals
}

func Pack[K comparable, V cmp.Ordered](keys []K, vals []V) (Pairs[K, V], error) {
	if len(keys) != len(vals) {
		return nil, fmt.Errorf("the lenght of the two slices must be the same")
	}

	p := make(Pairs[K, V], len(keys))
	for i, key := range keys {
		p[i].Key, p[i].Rank = key, vals[i]
	}

	return p, nil
}

// Top() returns the top `limit` Pairs, sorted by value. Worst case time complexity is O(|pairs| * limit).
func Top[K comparable, V cmp.Ordered](pairs Pairs[K, V], limit int) Pairs[K, V] {
	if limit < 1 || len(pairs) < 1 {
		return nil
	}

	if limit >= len(pairs) {
		sort.Sort(pairs)
		return pairs
	}

	top := pairs[:limit]
	i, min := top.Min()

	for _, pair := range pairs[limit:] {
		if pair.Rank > min {
			// swap out the smallest pair with the new one
			top[i] = pair
			i, min = top.Min()
		}
	}

	sort.Sort(top)
	return top
}
