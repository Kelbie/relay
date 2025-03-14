// The package pairs is useful for sorting, finding the top valued elements in a
// collection of key-value pairs.
package pairs

import (
	"cmp"
	"fmt"
	"sort"
)

type Pair[K comparable, V cmp.Ordered] struct {
	Key K
	Val V
}

type Pairs[K comparable, V cmp.Ordered] []Pair[K, V]

func (p Pairs[K, V]) Len() int           { return len(p) }
func (p Pairs[K, V]) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p Pairs[K, V]) Less(i, j int) bool { return p[i].Val > p[j].Val }

// Min() returns the position and value of the smallest pair by Val.
// In case of multiple candidates, the first one is chosen.
func (p Pairs[K, V]) Min() (int, V) {
	if len(p) < 1 {
		panic("dvm.Min: Pairs is empty")
	}

	index, min := 0, p[0].Val
	for i, pair := range p {
		if pair.Val < min {
			index = i
			min = pair.Val
		}
	}
	return index, min
}

// Max() returns the position and value of the biggest pair by Val.
// In case of multiple candidates, the first one is chosen.
func (p Pairs[K, V]) Max() (int, V) {
	if len(p) < 1 {
		panic("dvm.Max: Pairs is empty")
	}

	index, max := 0, p[0].Val
	for i, pair := range p {
		if pair.Val > max {
			index = i
			max = pair.Val
		}
	}
	return index, max
}

func (p Pairs[K, V]) Unpack() ([]K, []V) {
	keys := make([]K, len(p))
	vals := make([]V, len(p))

	for i, pair := range p {
		keys[i] = pair.Key
		vals[i] = pair.Val
	}

	return keys, vals
}

func Pack[K comparable, V cmp.Ordered](keys []K, vals []V) (Pairs[K, V], error) {
	if len(keys) != len(vals) {
		return nil, fmt.Errorf("the lenght of the two slices must be the same")
	}

	p := make(Pairs[K, V], len(keys))
	for i, key := range keys {
		p[i].Key, p[i].Val = key, vals[i]
	}

	return p, nil
}

// Top() returns the top `limit` Pairs, sorted by value. Worst case time complexity is O(|pairs| * limit).
func (p Pairs[K, V]) Top(limit int) Pairs[K, V] {
	if limit < 1 || len(p) < 1 {
		return nil
	}

	if limit >= len(p) {
		sort.Sort(p)
		return p
	}

	top := p[:limit]
	i, min := top.Min()

	for _, pair := range p[limit:] {
		if pair.Val > min {
			// swap out the smallest pair with the new one
			top[i] = pair
			i, min = top.Min()
		}
	}

	sort.Sort(top)
	return top
}

// Top() extracts the top `limit` key-value pairs from the map, sorted by value.
// For proper cases (0 < limit << |m|), the worst case time complexity is O(|m| * limit).
func Top[K comparable, V cmp.Ordered](m map[K]V, limit int) Pairs[K, V] {
	if limit < 1 || len(m) < 1 {
		return nil
	}

	if limit >= len(m) {
		pairs := ToPairs(m)
		sort.Sort(pairs)
		return pairs
	}

	top := make(Pairs[K, V], 0, limit)
	first := true
	var i int
	var min V

	for key, val := range m {
		if len(top) < limit {
			top = append(top, Pair[K, V]{Key: key, Val: val})
			continue
		}

		if first {
			// only when it's full, compute the min
			first = false
			i, min = top.Min()
		}

		if val > min {
			// swap out the smallest pair with current key value pair
			top[i] = Pair[K, V]{Key: key, Val: val}
			i, min = top.Min()
		}
	}

	sort.Sort(top)
	return top
}

// ToPairs() converts the entire map into a slice of key-value pairs.
func ToPairs[K comparable, V cmp.Ordered](m map[K]V) Pairs[K, V] {
	pairs := make(Pairs[K, V], 0, len(m))
	for key, val := range m {
		pairs = append(pairs, Pair[K, V]{Key: key, Val: val})
	}

	return pairs
}
