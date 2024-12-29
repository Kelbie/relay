package dvm

import (
	"context"
	"errors"
	"math/rand/v2"
	"reflect"
	"testing"

	mockdb "github.com/vertex-lab/crawler/pkg/database/mock"
	mockstore "github.com/vertex-lab/crawler/pkg/store/mock"
)

func TestRelevantWhoFollow(t *testing.T) {
	testCases := []struct {
		name          string
		DBType        string
		RWSType       string
		args          *Args
		expectedRes   []RankResponse
		expectedError error
	}{
		{
			name:          "nil args",
			DBType:        "simple-with-mock-pks",
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
			name:    "valid global",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &Args{
				Source:  odell,
				Targets: []string{calle},
				Limit:   1,
				Sort:    "global",
			},
			expectedError: nil,
			expectedRes:   []RankResponse{{Pubkey: odell, Rank: 0.5}},
		},
		{
			name:    "valid personalized",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &Args{
				Source:  odell,
				Targets: []string{calle},
				Limit:   1,
				Sort:    "personalized",
			},
			expectedError: nil,
			expectedRes:   []RankResponse{{Pubkey: odell, Rank: 0.54054}},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			DB := mockdb.SetupDB(test.DBType)
			RWS := mockstore.SetupRWS(test.RWSType)
			res, err := RelevantWhoFollow(ctx, DB, RWS, test.args)

			if !errors.Is(err, test.expectedError) {
				t.Fatalf("RelevantWhoFollow: expected error %v, got %v", test.expectedError, err)
			}

			const maxDist float64 = 0.001
			dist := ResponseDistance(res, test.expectedRes)
			if dist > maxDist {
				t.Errorf("RelevantWhoFollow: expected distance %v, got %v", maxDist, dist)
				t.Errorf("expected response %v, got %v", test.expectedRes, res)
			}
		})
	}
}

func TestTopNByValue(t *testing.T) {
	testCases := []struct {
		name         string
		m            map[uint32]float64
		topN         uint64
		expectedKeys []uint32
		expectedVals []float64
	}{
		{
			name:         "nil map",
			m:            nil,
			topN:         1,
			expectedKeys: nil,
			expectedVals: nil,
		},
		{
			name:         "empty map",
			m:            map[uint32]float64{},
			topN:         1,
			expectedKeys: nil,
			expectedVals: nil,
		},
		{
			name:         "topN = 0",
			m:            map[uint32]float64{0: 1.0},
			topN:         0,
			expectedKeys: nil,
			expectedVals: nil,
		},
		{
			name:         "topN bigger than the map",
			m:            map[uint32]float64{0: 1.0},
			topN:         10,
			expectedKeys: nil,
			expectedVals: nil,
		},
		{
			name:         "valid, just one",
			m:            map[uint32]float64{0: 1.0, 1: 2.2},
			topN:         1,
			expectedKeys: []uint32{1},
			expectedVals: []float64{2.2},
		},
		{
			name:         "valid",
			m:            map[uint32]float64{0: 1.0, 1: 2.2, 2: 11.2, 4: 0.76},
			topN:         3,
			expectedKeys: []uint32{2, 1, 0},
			expectedVals: []float64{11.2, 2.2, 1.0},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			keys, vals := TopByValue(test.m, test.topN)

			if !reflect.DeepEqual(keys, test.expectedKeys) {
				t.Fatalf("TopByValue: expected keys %v, got %v", test.expectedKeys, keys)
			}

			if !reflect.DeepEqual(vals, test.expectedVals) {
				t.Fatalf("TopByValue: expected vals %v, got %v", test.expectedVals, vals)
			}
		})
	}
}

// -----------------------------------BENCHMARKS--------------------------------

func BenchmarkTopNByValue(b *testing.B) {
	const mapSize int = 1000000
	m := make(map[uint32]float64, mapSize)

	for i := 0; i < mapSize; i++ {
		m[rand.Uint32()] = rand.ExpFloat64()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		TopByValue(m, 100)
	}
}
