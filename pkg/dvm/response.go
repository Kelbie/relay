package dvm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/vertex-lab/crawler/pkg/models"
	"github.com/vertex-lab/crawler/pkg/pagerank"
)

// RankResponse (for type) returns the rank for the requested pubkey.
type RankResponse struct {
	Pubkey string  `json:"pubkey"`
	Rank   float64 `json:"rank"`
}

// VerifyReputation() returns the rank of the target and its highest ranked followers.
// All ranks use the specified args.Sort algorithm.
func VerifyReputation(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *Args) ([]RankResponse, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := validateVerifyReputation(args); err != nil {
		return nil, fmt.Errorf("VerifyReputation: %w", err)
	}

	IDs, err := DB.NodeIDs(ctx, args.Targets[0], args.Source)
	if err != nil {
		return nil, fmt.Errorf("VerifyReputation: failed to fetch the IDs of source and target: %w", err)
	}

	target, source := IDs[0], IDs[1]
	if target == nil {
		// if target is not found in our database, we assume it's a low-reputation key. This heuristic is based
		// on the fact that to be added to our DB, a key requires only one follows from an active node.
		return []RankResponse{{Pubkey: args.Targets[0], Rank: 0}}, nil
	}

	followers, err := DB.Followers(ctx, *target)
	if err != nil {
		return nil, fmt.Errorf("VerifyReputation: failed to fetch the followers of target: %w", err)
	}

	toRank := append(followers[0], *target)
	rankMap, err := rankNodes(ctx, DB, RWS, toRank, args.Sort, source)
	if err != nil {
		return nil, fmt.Errorf("VerifyReputation: %w", err)
	}

	// Target must be first in the response, then its highest ranked followers.
	// To avoid the possibility of target being in the second group as well, we remove its key from the rankMap.
	nodeRanks := pairs{{ID: *target, rank: rankMap[*target]}}
	delete(rankMap, *target)
	nodeRanks = append(nodeRanks, topPairs(rankMap, args.Limit)...)

	res, err := buildResponse(ctx, DB, nodeRanks)
	if err != nil {
		return nil, fmt.Errorf("VerifyReputation: %w", err)
	}

	return res, nil
}

func SortAuthors(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *Args) ([]RankResponse, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := validateSortAuthors(args); err != nil {
		return nil, fmt.Errorf("SortAuthors: %w", err)
	}

	args.Limit = min(args.Limit, len(args.Targets))

	IDs, err := DB.NodeIDs(ctx, append(args.Targets, args.Source)...)
	if err != nil {
		return nil, fmt.Errorf("SortAuthors: failed to fetch the IDs of source and targets: %w", err)
	}

	var sourceID *uint32 = IDs[len(IDs)-1]
	var targetIDs []uint32
	var notFound []string
	for i := 0; i < len(IDs)-1; i++ {
		if IDs[i] != nil {
			targetIDs = append(targetIDs, *IDs[i])
		} else {
			notFound = append(notFound, args.Targets[i])
		}
	}

	ranks, err := rankNodes(ctx, DB, RWS, targetIDs, args.Sort, sourceID)
	if err != nil {
		return nil, fmt.Errorf("SortAuthors: %w", err)
	}

	top := topPairs(ranks, args.Limit)
	res, err := buildResponse(ctx, DB, top)
	if err != nil {
		return nil, fmt.Errorf("SortAuthors: %w", err)
	}

	var i int
	for len(res) < args.Limit {
		// if the response is shorter than `limit`, "pad" it with pubkeys that are not found
		// as always, the assumption is that, if a key was not found in our DB, it's not reputable
		res = append(res, RankResponse{Pubkey: notFound[i], Rank: 0.0})
		i++
	}

	return res, nil
}

// RecommendFollows() uses the specified args.Sort algorithm to provide a list of recommendations to args.Source
func RecommendFollows(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *Args) ([]RankResponse, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := validateRecommendFollows(args); err != nil {
		return nil, fmt.Errorf("RecommendFollows: %w", err)
	}

	var ranks models.PagerankMap
	var err error
	switch args.Sort {
	case "global":
		ranks, err = recommendFollowsGlobal(ctx, DB, RWS, args.Source)

	case "personalized":
		limit := int(min(args.Limit, 30))
		ranks, err = recommendFollowsPersonalized(ctx, DB, RWS, args.Source, limit)

	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidSortOption, args.Sort)
	}

	if err != nil {
		return nil, fmt.Errorf("RecommendFollows: %w", err)
	}

	top := topPairs(ranks, int(args.Limit))
	res, err := buildResponse(ctx, DB, top)
	if err != nil {
		return nil, fmt.Errorf("RecommendFollows: %w", err)
	}

	return res, nil
}

// rankNodes() associates a rank to each target by applying the specified algorithm.
// If the algorithm is personalized, it uses the provided source.
func rankNodes(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	targets []uint32,
	algorithm string,
	source *uint32) (models.PagerankMap, error) {

	if len(targets) == 0 {
		return nil, fmt.Errorf("%w: empty targets", ErrInvalidTargets)
	}

	var ranks models.PagerankMap
	var err error

	switch algorithm {
	case "global":
		ranks, err = pagerank.Global(ctx, RWS, targets...)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to sort with global pagerank %v", ErrComputationFailed, err)
		}

	case "personalized":
		if source == nil {
			return nil, fmt.Errorf("source %w", ErrKeyNotFound)
		}

		pp, err := pagerank.Personalized(ctx, DB, RWS, *source, 100)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to sort with personalized pagerank %v", ErrComputationFailed, err)
		}

		ranks = make(models.PagerankMap, len(targets))
		for _, ID := range targets {
			ranks[ID] = pp[ID]
		}

	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidSortOption, algorithm)
	}

	return ranks, nil
}

func recommendFollowsGlobal(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	source string) (models.PagerankMap, error) {

	var avoid []uint32 // a slice of nodeIDs that should not be recommended, like self, follows, mutes...
	node, err := DB.NodeByKey(ctx, source)

	switch {
	case errors.Is(err, models.ErrNodeNotFoundDB):
		// do nothing, as we can still recommend.

	case err != nil:
		// if the error is different than node-not-found it means there are issue with our DB, so it's better to fail.
		return nil, fmt.Errorf("failed to fetch the ID of source: %w", err)

	case node == nil:
		// if there is no error and the node is nil, it means there are issue with our DB, so it's better to fail.
		return nil, fmt.Errorf("failed to fetch the ID of source: node is nil")

	default:
		follows, err := DB.Follows(ctx, node.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch follows of %s", source)
		}
		// remove follows and self from the recommendations
		avoid = append(follows[0], node.ID)
	}

	// this should be faster than using DB.AllNodes(). It might happen that some nodeIDs
	// are not associated with any node, but this is not a problem since their pagerank will be 0.
	size := DB.Size(ctx)
	candidates := make([]uint32, size)
	for i := 0; i < size; i++ {
		candidates[i] = uint32(i)
	}

	ranks, err := pagerank.Global(ctx, RWS, candidates...)
	if err != nil {
		return nil, fmt.Errorf("failed to recommend with global pagerank: %w", err)
	}

	for _, ID := range avoid {
		delete(ranks, ID)
	}

	return ranks, nil
}

func recommendFollowsPersonalized(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	source string,
	limit int) (models.PagerankMap, error) {

	var avoid []uint32 // a slice of nodeIDs that should not be recommended, like self, follows, mutes...
	node, err := DB.NodeByKey(ctx, source)

	switch {
	case err != nil:
		return nil, fmt.Errorf("failed to fetch the ID of source: %w", err)

	case node == nil:
		return nil, fmt.Errorf("failed to fetch the ID of source: node is nil")

	case node.Status == models.StatusInactive:
		// if the node is inactive, we don't have reliable data thus we prefer not to give any recommendation
		return nil, fmt.Errorf("we don't have reliable data for the source %s", source)

	default:
		follows, err := DB.Follows(ctx, node.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch follows of %s", source)
		}
		// remove follows and self from the recommendations
		avoid = append(follows[0], node.ID)
	}

	ranks, err := pagerank.Personalized(ctx, DB, RWS, node.ID, uint16(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to recommend with personalized pagerank: %w", err)
	}

	for _, ID := range avoid {
		delete(ranks, ID)
	}

	return ranks, nil
}

func validateVerifyReputation(args *Args) error {
	if args == nil {
		return ErrNilArgs
	}

	if len(args.Targets) != 1 {
		return fmt.Errorf("%w: exactly one target must be provided for VerifyReputation", ErrInvalidTargets)
	}

	if args.Limit < 1 {
		return fmt.Errorf("%w: limit must be greater than one", ErrInvalidLimit)
	}

	return nil
}

func validateRecommendFollows(args *Args) error {
	if args == nil {
		return ErrNilArgs
	}

	if args.Limit < 1 {
		return fmt.Errorf("%w: limit must be  greater than one", ErrInvalidLimit)
	}

	return nil
}

func validateSortAuthors(args *Args) error {
	if args == nil {
		return ErrNilArgs
	}

	if len(args.Targets) < 1 {
		return fmt.Errorf("%w: at least one target must be provided for SortAuthors", ErrInvalidTargets)
	}

	if args.Limit < 1 {
		return fmt.Errorf("%w: limit must be greater than one", ErrInvalidLimit)
	}

	return nil
}

// buildResponse() replaces IDs with pubkeys and returns RankResponses
func buildResponse(ctx context.Context, DB models.Database, nodeRanks pairs) ([]RankResponse, error) {
	if len(nodeRanks) < 1 {
		// if nodeRanks is empty, we return an empty response. This can happen for example when calling
		// SortAuthors and all args.Targets are not present in our DB.
		return []RankResponse{}, nil
	}

	IDs, ranks := nodeRanks.Unpack()
	pubkeys, err := DB.Pubkeys(ctx, IDs...)
	if err != nil {
		return nil, fmt.Errorf("%w: buildResponse: failed to convert nodeIDs to pubkeys: %w", ErrComputationFailed, err)
	}

	res := make([]RankResponse, len(nodeRanks))
	for i, pk := range pubkeys {
		if pk == nil {
			return nil, fmt.Errorf("%w: buildResponse: %w ID=%d", ErrComputationFailed, models.ErrNodeNotFoundDB, IDs[i])
		}

		res[i] = RankResponse{Pubkey: *pk, Rank: ranks[i]}
	}

	return res, nil
}

// -----------------------------------HELPERS-----------------------------------

type pair struct {
	ID   uint32
	rank float64
}

type pairs []pair

func (p pairs) Len() int           { return len(p) }
func (p pairs) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p pairs) Less(i, j int) bool { return p[i].rank > p[j].rank }
func (p pairs) Min() (int, float64) {
	if len(p) < 1 {
		panic("dvm.Min: pairs is empty")
	}
	index, min := 0, p[0].rank
	for i, pair := range p {
		if pair.rank < min {
			index = i
			min = pair.rank
		}
	}
	return index, min
}

func (p pairs) Unpack() ([]uint32, []float64) {
	IDs := make([]uint32, len(p))
	ranks := make([]float64, len(p))

	for i, pair := range p {
		IDs[i] = pair.ID
		ranks[i] = pair.rank
	}

	return IDs, ranks
}

// topPairs() returns a slice of `limit` (key, value), sorted by value. Worst case time complexity is O(len(m) * limit)
func topPairs(m models.PagerankMap, limit int) pairs {
	l := min(limit, len(m))
	if l < 1 {
		return nil
	}

	var i int
	var min float64
	pairs := make(pairs, 0, l)

	for ID, rank := range m {
		if len(pairs) < l {
			// append the first l pairs
			pairs = append(pairs, pair{ID: ID, rank: rank})
			i, min = pairs.Min()
			continue
		}

		if rank > min {
			// swap out the smallest pair with the new one
			pairs[i] = pair{ID: ID, rank: rank}
			i, min = pairs.Min()
		}
	}

	sort.Sort(pairs)
	return pairs
}

// inefficientTopPairs() is only used to test topPairs by comparing their results.
func inefficientTopPairs(m models.PagerankMap, limit int) pairs {
	l := min(limit, len(m))
	if l < 1 {
		return nil
	}

	pairs := make(pairs, 0, len(m))
	for ID, rank := range m {
		pairs = append(pairs, pair{ID: ID, rank: rank})
	}

	sort.Sort(pairs)
	return pairs[:l]
}

// ResponseDistance() returns the L1 distance between two RankResponses.
func ResponseDistance(res1, res2 []RankResponse) float64 {
	if len(res1) != len(res2) {
		return math.MaxFloat64
	}

	// sort the responses in lexicographic order of the keys before comparing
	sort.Slice(res1, func(i, j int) bool { return res1[i].Pubkey > res1[j].Pubkey })
	sort.Slice(res2, func(i, j int) bool { return res2[i].Pubkey > res2[j].Pubkey })

	var distance float64
	for i := range res1 {
		if res1[i].Pubkey != res2[i].Pubkey {
			// if the keys are different, the two responses are incomparable
			return math.MaxFloat64
		}

		distance += math.Abs(res1[i].Rank - res2[i].Rank)
	}

	return distance
}
