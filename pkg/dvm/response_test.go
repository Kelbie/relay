package dvm

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"testing"

	mockdb "github.com/vertex-lab/crawler/pkg/database/mock"
	mockstore "github.com/vertex-lab/crawler/pkg/store/mock"
	"github.com/vertex-lab/relay/pkg/eventstore"
)

const maxDist float64 = 0.002

func TestVerifyReputation(t *testing.T) {
	tests := []struct {
		name     string
		DBType   string
		RWSType  string
		args     *VerifyReputationArgs
		expected PubkeyRanks
	}{
		{
			name:    "target not in the DB",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &VerifyReputationArgs{
				Algorithm: Algorithm{Sort: Global},
				Target:    randomKey,
				Limit:     5,
			},
			expected: PubkeyRanks{{Key: randomKey, Rank: 0}},
		},
		{
			name:    "valid global (simple)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &VerifyReputationArgs{
				Algorithm: Algorithm{Sort: Global},
				Target:    calle,
				Limit:     1,
			},
			expected: PubkeyRanks{{Key: calle, Rank: 0.5}, {Key: odell, Rank: 0.5}},
		},
		{
			name:    "valid global (triangle)",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &VerifyReputationArgs{
				Algorithm: Algorithm{Sort: Global},
				Target:    "2",
				Limit:     1,
			},
			expected: PubkeyRanks{{Key: "2", Rank: 0.33333}, {Key: "1", Rank: 0.33333}},
		},
		{
			name:    "valid personalized (simple)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &VerifyReputationArgs{
				Algorithm: Algorithm{Sort: Personalized, Source: odell},
				Target:    calle,
				Limit:     1,
			},
			expected: PubkeyRanks{{Key: calle, Rank: 0.45946}, {Key: odell, Rank: 0.54054}},
		},
		{
			name:    "valid personalized (triangle)",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &VerifyReputationArgs{
				Algorithm: Algorithm{Sort: Personalized, Source: "0"},
				Target:    "2",
				Limit:     1,
			},
			expected: PubkeyRanks{{Key: "2", Rank: 0.280855199}, {Key: "1", Rank: 0.330417881}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			DB := mockdb.SetupDB(test.DBType)
			RWS := mockstore.SetupRWS(test.RWSType)

			res, err := VerifyReputation(ctx, DB, RWS, test.args)
			if err != nil {
				t.Fatalf("expected error nil, got %v", err)
			}

			dist := distance(res, test.expected)
			if dist > maxDist {
				t.Errorf("VerifyReputation: expected distance %v, got %v", maxDist, dist)
				t.Errorf("expected response %v, got %v", test.expected, res)
			}
		})
	}
}

func TestSortProfiles(t *testing.T) {
	tests := []struct {
		name     string
		DBType   string
		RWSType  string
		args     *SortProfilesArgs
		expected PubkeyRanks
	}{
		{
			name:    "valid global (one target not found in the DB)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &SortProfilesArgs{
				Algorithm: Algorithm{Sort: Global},
				Targets:   []string{randomKey, calle, pip},
				Limit:     3,
			},
			expected: PubkeyRanks{{Key: calle, Rank: 0.5}, {Key: pip, Rank: 0.0}, {Key: randomKey, Rank: 0.0}},
		},
		{
			name:    "valid global (triangle)",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &SortProfilesArgs{
				Algorithm: Algorithm{Sort: Global},
				Targets:   []string{"0", "1", "2", "69"},
				Limit:     4,
			},
			expected: PubkeyRanks{{Key: "0", Rank: 0.33333}, {Key: "1", Rank: 0.33333}, {Key: "2", Rank: 0.33333}, {Key: "69", Rank: 0}},
		},
		{
			name:    "valid personalized (simple)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &SortProfilesArgs{
				Algorithm: Algorithm{Sort: Personalized, Source: odell},
				Targets:   []string{odell, calle, pip},
				Limit:     3,
			},
			expected: PubkeyRanks{{Key: odell, Rank: 0.540540541}, {Key: calle, Rank: 0.459459459}, {Key: pip, Rank: 0.0}},
		},
		{
			name:    "valid personalized (triangle)",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &SortProfilesArgs{
				Algorithm: Algorithm{Sort: Personalized, Source: "0"},
				Targets:   []string{"0", "1", "2"},
				Limit:     3,
			},
			expected: PubkeyRanks{{Key: "0", Rank: 0.388726919}, {Key: "1", Rank: 0.330417881}, {Key: "2", Rank: 0.280855199}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			DB := mockdb.SetupDB(test.DBType)
			RWS := mockstore.SetupRWS(test.RWSType)

			res, err := SortProfiles(ctx, DB, RWS, test.args)
			if err != nil {
				t.Fatalf("expected error nil, got %v", err)
			}

			dist := distance(res, test.expected)
			if dist > maxDist {
				t.Errorf("SortProfiles: expected distance %v, got %v", maxDist, dist)
				t.Errorf("expected response %v, got %v", test.expected, res)
			}
		})
	}
}

func TestSearchAuthors(t *testing.T) {
	tests := []struct {
		name     string
		DBType   string
		RWSType  string
		args     *SearchProfilesArgs
		expected PubkeyRanks
	}{
		{
			name:    "valid global",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &SearchProfilesArgs{
				Algorithm: Algorithm{Sort: Global},
				Search:    "pip",
				Limit:     5,
			},
			expected: PubkeyRanks{{Key: pip, Rank: 0.0}},
		},
		{
			name:    "valid global npub",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &SearchProfilesArgs{
				Algorithm: Algorithm{Sort: Global},
				Search:    "npub176p7sup477k5738qhxx0hk2n0cty2k5je5uvalzvkvwmw4tltmeqw7vgup",
				Limit:     5,
			},
			expected: PubkeyRanks{{Key: pip, Rank: 0.0}},
		},
		{
			name:    "valid global hex",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &SearchProfilesArgs{
				Algorithm: Algorithm{Sort: Global},
				Search:    pip,
				Limit:     5,
			},
			expected: PubkeyRanks{{Key: pip, Rank: 0.0}},
		},
		{
			name:    "valid no results",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &SearchProfilesArgs{
				Algorithm: Algorithm{Sort: Global},
				Search:    "jack",
				Limit:     5,
			},
		},
		{
			name:    "valid personalized (simple)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &SearchProfilesArgs{
				Algorithm: Algorithm{Sort: Personalized, Source: odell},
				Search:    "pip",
				Limit:     5,
			},
			expected: PubkeyRanks{{Key: pip, Rank: 0.0}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			DB := mockdb.SetupDB(test.DBType)
			RWS := mockstore.SetupRWS(test.RWSType)
			eventStore, err := eventstore.New("test.sqlite")
			if err != nil {
				t.Fatal(err)
			}

			res, err := SearchProfiles(ctx, DB, RWS, eventStore, test.args)
			if err != nil {
				t.Fatalf("expected error nil, got %v", err)
			}

			dist := distance(res, test.expected)
			if dist > maxDist {
				t.Errorf("SearchProfiles: expected distance %v, got %v", maxDist, dist)
				t.Errorf("expected response %v, got %v", test.expected, res)
			}
		})
	}
}

func TestRecommendFollows(t *testing.T) {
	tests := []struct {
		name     string
		DBType   string
		RWSType  string
		args     *RecommendFollowsArgs
		expected PubkeyRanks
	}{
		{
			name:    "valid global (simple)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &RecommendFollowsArgs{
				Algorithm: Algorithm{Sort: Global, Source: randomKey},
				Limit:     2,
			},
			expected: PubkeyRanks{{Key: calle, Rank: 0.5}, {Key: odell, Rank: 0.5}},
		},
		{
			name:    "valid global (triangle)",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &RecommendFollowsArgs{
				Algorithm: Algorithm{Sort: Global, Source: "0"},
				Limit:     1,
			},
			expected: PubkeyRanks{{Key: "2", Rank: 1.0 / 3.0}},
		},
		{
			name:    "valid personalized",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &RecommendFollowsArgs{
				Algorithm: Algorithm{Sort: Personalized, Source: "0"},
				Limit:     1,
			},
			expected: PubkeyRanks{{Key: "2", Rank: 0.2809}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			DB := mockdb.SetupDB(test.DBType)
			RWS := mockstore.SetupRWS(test.RWSType)

			res, err := RecommendFollows(ctx, DB, RWS, test.args)
			if err != nil {
				t.Fatalf("expected error nil, got %v", err)
			}

			dist := distance(res, test.expected)
			if dist > maxDist {
				t.Errorf("RecommendFollows: expected distance %v, got %v", maxDist, dist)
				t.Errorf("expected response %v, got %v", test.expected, res)
			}
		})
	}
}

// ---------------------------------BENCHMARKS----------------------------------

func BenchmarkConvertingMapToNodeRanks(b *testing.B) {
	sizes := []int{10000, 100000, 1000000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {

			m := make(map[uint32]float64, size)
			for i := 0; i < size; i++ {
				m[uint32(i)] = rand.Float64()
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// converting a map to a collection of NodeRanks
				n := make(NodeRanks, 0, size)
				for key, val := range m {
					n = append(n, Pair[uint32, float64]{Key: key, Rank: val})
				}
			}
		})
	}
}

// -----------------------------------HELPERS-----------------------------------

// distance() returns the L1 distance between two PubkeyRanks.
func distance(res1, res2 PubkeyRanks) float64 {
	if len(res1) != len(res2) {
		return math.MaxFloat64
	}

	// sort the responses in lexicographic order of the keys before comparing
	sort.Slice(res1, func(i, j int) bool { return res1[i].Key > res1[j].Key })
	sort.Slice(res2, func(i, j int) bool { return res2[i].Key > res2[j].Key })

	var distance float64
	for i := range res1 {
		if res1[i].Key != res2[i].Key {
			// if the keys are different, the two responses are incomparable
			return math.MaxFloat64
		}

		distance += math.Abs(res1[i].Rank - res2[i].Rank)
	}

	return distance
}
