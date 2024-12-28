package dvm

import (
	"context"
	"fmt"
	"sort"

	"github.com/vertex-lab/crawler/pkg/models"
	"github.com/vertex-lab/crawler/pkg/pagerank"
)

// RankResponse returns the requested rank for the pubkey
type RankResponse struct {
	Pubkey string  `json:"pubkey"`
	Rank   float64 `json:"rank"`
}

// type ImpersonatorResponse struct {
// 	Pubkey    string  `json:"pubkey"`
// 	Gpr       float64 `json:"gpr"`
// 	Ppr       float64 `json:"ppr"`
// 	Warning   bool    `json:"warning"`
// 	Candidate bool    `json:"candidate"`
// }

func RelevantWhoFollow(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *Args) ([]RankResponse, error) {

	if err := validateRelevantWhoFollow(args); err != nil {
		return nil, err
	}

	IDs, err := DB.NodeIDs(ctx, args.Source, args.Targets[0])
	if err != nil {
		return nil, err
	}

	if IDs[0] == nil {
		return nil, fmt.Errorf("%w: %v", ErrKeyNotFound, args.Source)
	}
	if IDs[1] == nil {
		return nil, fmt.Errorf("%w: %v", ErrKeyNotFound, args.Targets[0])
	}

	sourceID, targetID := *IDs[0], *IDs[1]
	followersByNode, err := DB.Followers(ctx, targetID)
	if err != nil {
		return nil, err
	}
	followers := followersByNode[0]

	var rankMap models.PagerankMap
	switch args.Sort {
	case "global":
		rankMap, err = pagerank.Global(ctx, RWS, followers...) // TODO, make a pagerank.Read() as computing it is overkill.
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrComputationFailed, err)
		}

	case "personalized":
		rankMap, err = pagerank.Personalized(ctx, DB, RWS, sourceID, 100)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrComputationFailed, err)
		}
	}

	nodeIDs, ranks := TopByValue(rankMap, args.Limit)
	pubkeys, err := DB.Pubkeys(ctx, nodeIDs...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrComputationFailed, err)
	}

	response := make([]RankResponse, len(pubkeys))
	for i, pk := range pubkeys {
		if pk == nil {
			return nil, fmt.Errorf("%w: %w: %v", ErrComputationFailed, models.ErrNodeNotFoundDB, nodeIDs[i])
		}

		response[i] = RankResponse{Pubkey: *pk, Rank: ranks[i]}
	}

	return response, nil
}

// ValidateRelevantWhoFollow() validates the arguments for RelevantWhoFollow.
func validateRelevantWhoFollow(args *Args) error {
	if args == nil {
		return ErrNilArgs
	}

	if len(args.Targets) != 1 {
		return fmt.Errorf("%w: exactly one target must be provided for relevant-who-follow", ErrInvalidTargets)
	}

	if args.Limit == 0 {
		return fmt.Errorf("%w: limit must be strictly greater than zero", ErrInvalidLimit)
	}

	return nil
}

// TopByValue() returns the keys and values of the topN pairs, sorted by value.
func TopByValue(m map[uint32]float64, topN uint64) (keys []uint32, vals []float64) {
	if m == nil || topN <= 0 || topN > uint64(len(m)) {
		return nil, nil
	}

	type kv struct {
		key uint32
		val float64
	}

	kvs := make([]kv, topN)
	for k, v := range m {
		if v <= kvs[topN-1].val {
			continue
		}

		// if it's bigger than the last of the top, add it and sort
		kvs[topN-1].key = k
		kvs[topN-1].val = v

		sort.Slice(kvs, func(i, j int) bool {
			return kvs[i].val > kvs[j].val
		})
	}

	keys = make([]uint32, topN)
	vals = make([]float64, topN)
	for i, kv := range kvs {
		keys[i] = kv.key
		vals[i] = kv.val
	}

	return keys, vals
}
