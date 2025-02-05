package dvm

import (
	"context"
	"errors"
	"math/rand/v2"
	"reflect"
	"testing"

	mockdb "github.com/vertex-lab/crawler/pkg/database/mock"
	"github.com/vertex-lab/crawler/pkg/models"
	mockstore "github.com/vertex-lab/crawler/pkg/store/mock"
)

func TestVerifyReputation(t *testing.T) {
	const maxDist float64 = 0.002
	testCases := []struct {
		name          string
		DBType        string
		RWSType       string
		args          *Args
		expectedRes   RankResponses
		expectedError error
	}{
		{
			name:          "nil args",
			DBType:        "simple-with-pks",
			RWSType:       "one-node0",
			args:          nil,
			expectedError: ErrNilArgs,
		},
		{
			name:    "args targets not one",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &Args{
				Source:  odell,
				Targets: []string{pip, calle},
			},
			expectedError: ErrInvalidTargets,
		},
		{
			name:    "args limit is zero",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &Args{
				Source:  odell,
				Targets: []string{calle},
				Limit:   0,
			},
			expectedError: ErrInvalidLimit,
		},
		{
			name:    "target not in the DB",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &Args{
				Source:  odell,
				Targets: []string{randomKey},
				Limit:   5,
			},
			expectedRes: RankResponses{{Pubkey: randomKey, Rank: 0.0}},
		},
		{
			name:    "valid global (simple)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &Args{
				Source:  odell,
				Targets: []string{calle},
				Limit:   1,
				Sort:    "globalPagerank",
			},
			expectedError: nil,
			expectedRes:   RankResponses{{Pubkey: calle, Rank: 0.5}, {Pubkey: odell, Rank: 0.5}},
		},
		{
			name:    "valid global (triangle)",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &Args{
				Source:  "0",
				Targets: []string{"2"},
				Limit:   1,
				Sort:    "globalPagerank",
			},
			expectedError: nil,
			expectedRes:   RankResponses{{Pubkey: "2", Rank: 0.33333}, {Pubkey: "1", Rank: 0.33333}},
		},
		{
			name:    "valid personalized (simple)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &Args{
				Source:  odell,
				Targets: []string{calle},
				Limit:   1,
				Sort:    "personalizedPagerank",
			},
			expectedError: nil,
			expectedRes:   RankResponses{{Pubkey: calle, Rank: 0.45946}, {Pubkey: odell, Rank: 0.54054}},
		},
		{
			name:    "valid personalized (triangle)",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &Args{
				Source:  "0",
				Targets: []string{"2"},
				Limit:   1,
				Sort:    "personalizedPagerank",
			},
			expectedError: nil,
			expectedRes:   RankResponses{{Pubkey: "2", Rank: 0.280855199}, {Pubkey: "1", Rank: 0.330417881}},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			DB := mockdb.SetupDB(test.DBType)
			RWS := mockstore.SetupRWS(test.RWSType)
			res, err := VerifyReputation(ctx, DB, RWS, test.args)

			if !errors.Is(err, test.expectedError) {
				t.Fatalf("VerifyReputation: expected error %v, got %v", test.expectedError, err)
			}

			dist := ResponseDistance(res, test.expectedRes)
			if dist > maxDist {
				t.Errorf("VerifyReputation: expected distance %v, got %v", maxDist, dist)
				t.Errorf("expected response %v, got %v", test.expectedRes, res)
			}
		})
	}
}

func TestRecommendFollows(t *testing.T) {
	const maxDist float64 = 0.02
	testCases := []struct {
		name          string
		DBType        string
		RWSType       string
		args          *Args
		expectedRes   RankResponses
		expectedError error
	}{
		{
			name:          "nil args",
			DBType:        "simple-with-pks",
			RWSType:       "one-node0",
			args:          nil,
			expectedError: ErrNilArgs,
		},
		{
			name:    "args limit is zero",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &Args{
				Source:  odell,
				Targets: []string{calle},
				Limit:   0,
			},
			expectedError: ErrInvalidLimit,
		},
		{
			name:    "valid global (source not in the DB)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &Args{
				Source: randomKey,
				Limit:  2,
				Sort:   "globalPagerank",
			},
			expectedError: nil,
			expectedRes:   RankResponses{{Pubkey: calle, Rank: 0.5}, {Pubkey: odell, Rank: 0.5}},
		},
		{
			name:    "valid global",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &Args{
				Source: "0",
				Limit:  1,
				Sort:   "globalPagerank",
			},
			expectedError: nil,
			expectedRes:   RankResponses{{Pubkey: "2", Rank: 1.0 / 3.0}},
		},
		{
			name:    "valid personalized",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &Args{
				Source: "0",
				Limit:  1,
				Sort:   "personalizedPagerank",
			},
			expectedError: nil,
			expectedRes:   RankResponses{{Pubkey: "2", Rank: 0.2809}},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			DB := mockdb.SetupDB(test.DBType)
			RWS := mockstore.SetupRWS(test.RWSType)
			res, err := RecommendFollows(ctx, DB, RWS, test.args)

			if !errors.Is(err, test.expectedError) {
				t.Fatalf("RecommendFollows: expected error %v, got %v", test.expectedError, err)
			}

			dist := ResponseDistance(res, test.expectedRes)
			if dist > maxDist {
				t.Errorf("RecommendFollows: expected distance %v, got %v", maxDist, dist)
				t.Errorf("expected response %v, got %v", test.expectedRes, res)
			}
		})
	}
}

func TestSortAuthors(t *testing.T) {
	const maxDist float64 = 0.002
	testCases := []struct {
		name          string
		DBType        string
		RWSType       string
		args          *Args
		expectedRes   RankResponses
		expectedError error
	}{
		{
			name:          "nil args",
			DBType:        "simple-with-pks",
			RWSType:       "one-node0",
			args:          nil,
			expectedError: ErrNilArgs,
		},
		{
			name:    "empty targets",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &Args{
				Source:  odell,
				Targets: []string{},
				Limit:   5,
			},
			expectedError: ErrInvalidTargets,
		},
		{
			name:    "args limit is zero",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &Args{
				Source:  odell,
				Targets: []string{calle},
				Limit:   0,
			},
			expectedError: ErrInvalidLimit,
		},
		{
			name:    "valid global (one target not found in the DB)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &Args{
				Source:  odell,
				Targets: []string{randomKey, calle, pip},
				Limit:   5,
				Sort:    "globalPagerank",
			},
			expectedError: nil,
			expectedRes:   RankResponses{{Pubkey: calle, Rank: 0.5}, {Pubkey: pip, Rank: 0.0}, {Pubkey: randomKey, Rank: 0.0}},
		},
		{
			name:    "valid global (simple)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &Args{
				Source:  odell,
				Targets: []string{calle, pip},
				Limit:   2,
				Sort:    "globalPagerank",
			},
			expectedError: nil,
			expectedRes:   RankResponses{{Pubkey: calle, Rank: 0.5}, {Pubkey: pip, Rank: 0.0}},
		},
		{
			name:    "valid global (triangle)",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &Args{
				Source:  "0",
				Targets: []string{"0", "1", "2"},
				Limit:   3,
				Sort:    "globalPagerank",
			},
			expectedError: nil,
			expectedRes:   RankResponses{{Pubkey: "0", Rank: 0.33333}, {Pubkey: "1", Rank: 0.33333}, {Pubkey: "2", Rank: 0.33333}},
		},
		{
			name:    "valid personalized (simple)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &Args{
				Source:  odell,
				Targets: []string{odell, calle, pip},
				Limit:   3,
				Sort:    "personalizedPagerank",
			},
			expectedError: nil,
			expectedRes:   RankResponses{{Pubkey: odell, Rank: 0.540540541}, {Pubkey: calle, Rank: 0.459459459}, {Pubkey: pip, Rank: 0.0}},
		},
		{
			name:    "valid personalized (triangle)",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &Args{
				Source:  "0",
				Targets: []string{"0", "1", "2"},
				Limit:   3,
				Sort:    "personalizedPagerank",
			},
			expectedError: nil,
			expectedRes:   RankResponses{{Pubkey: "0", Rank: 0.388726919}, {Pubkey: "1", Rank: 0.330417881}, {Pubkey: "2", Rank: 0.280855199}},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			DB := mockdb.SetupDB(test.DBType)
			RWS := mockstore.SetupRWS(test.RWSType)
			res, err := SortAuthors(ctx, DB, RWS, test.args)

			if !errors.Is(err, test.expectedError) {
				t.Fatalf("SortAuthors: expected error %v, got %v", test.expectedError, err)
			}

			dist := ResponseDistance(res, test.expectedRes)
			if dist > maxDist {
				t.Errorf("SortAuthors: expected distance %v, got %v", maxDist, dist)
				t.Errorf("expected response %v, got %v", test.expectedRes, res)
			}
		})
	}
}

func TestTopPairs(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		testCases := []struct {
			name          string
			m             map[uint32]float64
			limit         int
			expectedPairs pairs
		}{
			{
				name:  "nil map",
				limit: 1,
			},
			{
				name:  "empty map",
				m:     map[uint32]float64{},
				limit: 1,
			},
			{
				name:  "limit = 0",
				m:     map[uint32]float64{0: 1.0},
				limit: 0,
			},
			{
				name:          "limit bigger than the map",
				m:             map[uint32]float64{0: 1.0},
				limit:         10,
				expectedPairs: pairs{{ID: 0, rank: 1}},
			},
			{
				name:          "one zero value",
				m:             map[uint32]float64{1: 1.0, 2: 0.0},
				limit:         2,
				expectedPairs: pairs{{ID: 1, rank: 1}, {ID: 2, rank: 0}},
			},
			{
				name:          "valid, just one",
				m:             map[uint32]float64{0: 1.0, 1: 2.2},
				limit:         1,
				expectedPairs: pairs{{ID: 1, rank: 2.2}},
			},
			{
				name:          "valid",
				m:             map[uint32]float64{0: 1.0, 1: 2.2, 4: 0.76, 2: 11.2, 11: 0.022},
				limit:         3,
				expectedPairs: pairs{{ID: 2, rank: 11.2}, {ID: 1, rank: 2.2}, {ID: 0, rank: 1.0}},
			},
		}

		for _, test := range testCases {
			t.Run(test.name, func(t *testing.T) {
				pairs := topPairs(test.m, test.limit)
				if !reflect.DeepEqual(pairs, test.expectedPairs) {
					t.Fatalf("TopByValue: expected pairs %v, got %v", test.expectedPairs, pairs)
				}
			})
		}
	})

	t.Run("fuzzy", func(t *testing.T) {
		const iter = 100
		const maxSize = 1000

		for i := 0; i < iter; i++ {
			// build a random map of random size
			mapSize := rand.IntN(maxSize)
			limit := rand.IntN(maxSize)
			m := make(models.PagerankMap, mapSize)
			for j := 0; j < mapSize; j++ {
				m[uint32(j)] = rand.Float64()
			}

			pairs := topPairs(m, limit)
			expectedPairs := inefficientTopPairs(m, limit)

			if !reflect.DeepEqual(pairs, expectedPairs) {
				t.Errorf("len(map) = %d; limit = %d", len(m), limit)
				t.Fatalf("got %v, expected %v", pairs, expectedPairs)
			}
		}
	})
}

// -----------------------------------BENCHMARKS--------------------------------

func BenchmarkTopPairs(b *testing.B) {
	testCases := []struct {
		name  string
		limit int
	}{
		{name: "top 10", limit: 10},
		{name: "top 100", limit: 100},
		{name: "top 250", limit: 250},
		{name: "top 500", limit: 500},
		{name: "top 750", limit: 750},
		{name: "top 1000", limit: 1000},
	}

	const mapSize int = 1000000
	m := make(map[uint32]float64, mapSize)

	for i := 0; i < mapSize; i++ {
		m[rand.Uint32()] = rand.ExpFloat64()
	}

	for _, test := range testCases {
		b.Run(test.name, func(b *testing.B) {
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				topPairs(m, test.limit)
			}
		})
	}
}

func BenchmarkInefficientTopPairs(b *testing.B) {
	testCases := []struct {
		name  string
		limit int
	}{
		{name: "top 10", limit: 10},
		{name: "top 100", limit: 100},
		{name: "top 250", limit: 250},
		{name: "top 500", limit: 500},
		{name: "top 750", limit: 750},
		{name: "top 1000", limit: 1000},
	}

	const mapSize int = 1000000
	m := make(map[uint32]float64, mapSize)

	for i := 0; i < mapSize; i++ {
		m[rand.Uint32()] = rand.ExpFloat64()
	}

	for _, test := range testCases {
		b.Run(test.name, func(b *testing.B) {
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				inefficientTopPairs(m, test.limit)
			}
		})
	}
}
