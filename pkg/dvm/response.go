package dvm

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/vertex-lab/crawler/pkg/models"
	"github.com/vertex-lab/crawler/pkg/pagerank"
)

// RankResponse (for type) returns the requested rank for the pubkey.
type RankResponse struct {
	Pubkey string  `json:"pubkey"`
	Rank   float64 `json:"rank"`
}

// RelevantWhoFollow() returns `limit` RankResponses for "relevant" pubkeys
// that follow the specified `target`. These relevant pubkeys are the ones with
// the highest scores among the followers of the target, determined by the specified
// sorting algorithm (e.g. personalized pagerank).
func RelevantWhoFollow(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *Args) ([]RankResponse, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

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

	// if target has no followers, return an empty response
	followers := followersByNode[0]
	if len(followers) == 0 {
		return []RankResponse{}, nil
	}

	var followersRank models.PagerankMap
	switch args.Sort {
	case "global":
		followersRank, err = pagerank.Global(ctx, RWS, followers...)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrComputationFailed, err)
		}

	case "personalized":
		pp, err := pagerank.Personalized(ctx, DB, RWS, sourceID, 100)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrComputationFailed, err)
		}

		followersRank = make(models.PagerankMap, len(followers))
		for _, follower := range followers {
			followersRank[follower] = pp[follower]
		}
	}

	return ResponseFromMap(ctx, DB, followersRank, args.Limit)
}

func RecommendedFollows(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *Args) ([]RankResponse, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := validateRecommendedFollows(args); err != nil {
		return nil, err
	}

	IDs, err := DB.NodeIDs(ctx, args.Source)
	if err != nil {
		return nil, err
	}
	if IDs[0] == nil {
		return nil, fmt.Errorf("%w: %v", ErrKeyNotFound, args.Source)
	}
	sourceID := *IDs[0]

	followsByNode, err := DB.Follows(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	follows := followsByNode[0]

	var candidatesRank models.PagerankMap
	switch args.Sort {
	case "global":
		// anyone is a candidate; TODO: we should smart in constraining the set of candidates in ways that make sense.
		candidates, err := DB.AllNodes(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrComputationFailed, err)
		}

		candidatesRank, err = pagerank.Global(ctx, RWS, candidates...)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrComputationFailed, err)
		}

	case "personalized":
		limit := min(args.Limit, 10)
		candidatesRank, err = pagerank.Personalized(ctx, DB, RWS, sourceID, uint16(limit))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrComputationFailed, err)
		}
	}

	// zeroing the rank of self and follows, so they don't get recommended.
	candidatesRank[sourceID] = 0.0
	for _, follow := range follows {
		candidatesRank[follow] = 0.0
	}

	return ResponseFromMap(ctx, DB, candidatesRank, args.Limit)
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

// ValidateRecommendedFollows() validates the arguments for RecommendedFollows.
func validateRecommendedFollows(args *Args) error {
	if args == nil {
		return ErrNilArgs
	}

	if args.Limit == 0 {
		return fmt.Errorf("%w: limit must be strictly greater than zero", ErrInvalidLimit)
	}

	return nil
}

// -----------------------------------HELPERS-----------------------------------

// ResponseFromMap() returns a slice of RankResponses from the given pagerank map nodeID --> rank.
func ResponseFromMap(
	ctx context.Context,
	DB models.Database,
	rankMap models.PagerankMap,
	limit uint64) ([]RankResponse, error) {

	nodeIDs, ranks := TopByValue(rankMap, limit)
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

// TopByValue() returns the keys and values of the topN pairs, sorted by value.
func TopByValue(m map[uint32]float64, topN uint64) (keys []uint32, vals []float64) {
	if len(m) == 0 || topN <= 0 {
		return nil, nil
	}

	if topN > uint64(len(m)) {
		topN = uint64(len(m))
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

		// if it's bigger than the smallest of the top, add it and sort
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

// ResponseDistance() returns the L1 distance between two RankResponses.
func ResponseDistance(res1, res2 []RankResponse) float64 {
	if len(res1) != len(res2) {
		return math.MaxFloat64
	}

	var distance float64
	for i := range res1 {
		if res1[i].Pubkey != res2[i].Pubkey {
			return math.MaxFloat64
		}

		distance += math.Abs(res1[i].Rank - res2[i].Rank)
	}

	return distance
}
