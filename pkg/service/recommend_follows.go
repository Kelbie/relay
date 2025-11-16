package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/slicex"
	"github.com/vertex-lab/crawler_v2/pkg/graph"
	"github.com/vertex-lab/crawler_v2/pkg/pagerank"
)

var (
	RecommendFollowsSorts = []string{Global, Personalized, Followers}
	RecommendFollowsLimit = 100

	ErrUnsuportedRecommendFollows = errors.New("param must be one between 'source', 'sort', and 'limit'")
)

type RecommendFollowsArgs struct {
	Algorithm
	Limit int
}

func NewRecommendFollowsArgs(pubkey string) RecommendFollowsArgs {
	return RecommendFollowsArgs{
		Algorithm: Algorithm{
			Sort:   Global,
			Source: pubkey,
		},
		Limit: 5,
	}
}

func (a *RecommendFollowsArgs) Normalize() error {
	if a.Limit < 1 || a.Limit > RecommendFollowsLimit {
		return fmt.Errorf("%w: limit must be between 1 and %d: %d", ErrInvalidLimit, RecommendFollowsLimit, a.Limit)
	}

	if !slices.Contains(RecommendFollowsSorts, a.Sort) {
		return fmt.Errorf("%w: sort must be one between %v: %v", ErrInvalidSort, RecommendFollowsSorts, a.Sort)
	}

	if a.Sort == Personalized && !nostr.IsValidPublicKey(a.Source) {
		var err error
		a.Source, err = NpubToHex(a.Source)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidSource, err)
		}
	}
	return nil
}

// Cost returns the cost (measured in credits) of a service call with the provided arguments.
func (a RecommendFollowsArgs) Cost() int {
	if a.Sort == Personalized {
		return 10
	}
	return 1
}

type RecommendFollowsResult struct {
	Nodes           int
	Recommendations []Profile
}

// RecommendFollows uses the specified [Algorithm] to return a list of recommendations for args.Source.
// The recommended pubkeys are the highest ranked, excluding args.Source and its follows (if any).
// For more info read: https://vertexlab.io/docs/services/recommend-follows/
func (s *Service) RecommendFollows(ctx context.Context, args RecommendFollowsArgs) (RecommendFollowsResult, error) {
	response, err := s.recommendFollows(ctx, args)
	if err != nil {
		return RecommendFollowsResult{}, fmt.Errorf("RecommendFollows %w: %w", ErrInternal, err)
	}
	return response, nil
}

type nodeRanking = slicex.Pairs[graph.ID, float64] // a slice of (node ID, rank)

func (s *Service) recommendFollows(ctx context.Context, args RecommendFollowsArgs) (RecommendFollowsResult, error) {
	nodes, err := s.Redis.NodeCount(ctx)
	if err != nil {
		return RecommendFollowsResult{}, err
	}

	var candidates nodeRanking
	switch args.Sort {
	case Global:
		candidates, err = s.candidatesWithGlobal(ctx, args.Source)

	case Personalized:
		candidates, err = s.candidatesWithPersonalized(ctx, args.Source)

	case Followers:
		candidates, err = s.candidatesWithFollowers(ctx, args.Source)

	default:
		err = ErrInvalidSort
	}

	if err != nil {
		return RecommendFollowsResult{}, fmt.Errorf("failed to recommend with %s: %w", args.Sort, err)
	}

	recommendedNodes, ranks := candidates.MaxK(args.Limit).Unpack()
	recommendedPubkeys, err := s.Redis.Pubkeys(ctx, recommendedNodes...)
	if err != nil {
		return RecommendFollowsResult{}, err
	}

	response := RecommendFollowsResult{}
	response.Nodes = nodes
	response.Recommendations = make([]Profile, len(recommendedPubkeys))

	for i := range recommendedPubkeys {
		response.Recommendations[i].Pubkey = recommendedPubkeys[i]
		response.Recommendations[i].Rank = ranks[i]
	}
	return response, nil
}

func (s *Service) candidatesWithGlobal(ctx context.Context, source string) (nodeRanking, error) {
	avoid := make([]graph.ID, 0, 100) // pre-allocate

	node, err := s.Redis.NodeByKey(ctx, source)
	if err != nil && !errors.Is(err, graph.ErrNodeNotFound) {
		return nil, err
	}

	if !errors.Is(err, graph.ErrNodeNotFound) {
		// add the node representing the source and its follows to avoid,
		// so they can't get recommended.
		follows, err := s.Redis.Follows(ctx, node.ID)
		if err != nil {
			return nil, err
		}

		avoid = append(follows, node.ID)
	}

	// TODO: the working assumption is that the "first" 50k nodes contain
	// the highest ranked nodes. We should find better heuristics.
	candidates := make([]graph.ID, 50_000)
	for i := range 50_000 {
		candidates[i] = graph.ID(strconv.Itoa(i))
	}

	candidates = slicex.Difference(candidates, avoid)
	ranks, err := pagerank.Global(ctx, s.Redis, candidates...)
	if err != nil {
		return nil, err
	}

	return slicex.Pack(candidates, ranks), nil
}

func (s *Service) candidatesWithFollowers(ctx context.Context, source string) (nodeRanking, error) {
	avoid := make([]graph.ID, 0, 100) // pre-allocate

	node, err := s.Redis.NodeByKey(ctx, source)
	if err != nil && !errors.Is(err, graph.ErrNodeNotFound) {
		return nil, err
	}

	if !errors.Is(err, graph.ErrNodeNotFound) {
		// add the node representing the source and its follows to avoid,
		// so they can't get recommended.
		follows, err := s.Redis.Follows(ctx, node.ID)
		if err != nil {
			return nil, err
		}

		avoid = append(follows, node.ID)
	}

	// TODO: the working assumption is that the "first" 50k nodes contain
	// the highest ranked nodes. We should find better heuristics.
	candidates := make([]graph.ID, 50_000)
	for i := range 50_000 {
		candidates[i] = graph.ID(strconv.Itoa(i))
	}

	candidates = slicex.Difference(candidates, avoid)
	counts, err := s.Redis.FollowerCounts(ctx, candidates...)
	if err != nil {
		return nil, err
	}

	nodeRanking := make(nodeRanking, len(candidates))
	for i := range candidates {
		nodeRanking[i].Key = candidates[i]
		nodeRanking[i].Val = float64(counts[i])
	}
	return nodeRanking, nil
}

func (s *Service) candidatesWithPersonalized(ctx context.Context, source string) (nodeRanking, error) {
	node, err := s.Redis.NodeByKey(ctx, source)
	if err != nil {
		return nil, err
	}

	follows, err := s.Redis.Follows(ctx, node.ID)
	if err != nil {
		return nil, err
	}

	pp, err := pagerank.Personalized(ctx, s.Redis, node.ID, 100_000)
	if err != nil {
		return nil, err
	}

	delete(pp, node.ID)
	for _, ID := range follows {
		delete(pp, ID)
	}
	return slicex.ToPairs(pp), nil
}
