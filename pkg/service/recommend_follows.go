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
)

type RecommendFollowsArgs struct {
	Algorithm
	Limit int
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

type RecommendFollowsItem struct {
	Pubkey string  `json:"pubkey"`
	Rank   float64 `json:"rank"`
}

type RecommendFollowsResponse []RecommendFollowsItem

// RecommendFollows uses the specified [Algorithm] to return a list of recommendations for args.Source.
// The recommended pubkeys are the highest ranked, excluding args.Source and its follows (if any).
// For more info read: https://vertexlab.io/docs/services/recommend-follows/
func (s *Service) RecommendFollows(ctx context.Context, args RecommendFollowsArgs) (RecommendFollowsResponse, error) {
	response, err := s.recommendFollows(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("RecommendFollows %w: %w", ErrInternal, err)
	}
	return response, nil
}

func (s *Service) recommendFollows(ctx context.Context, args RecommendFollowsArgs) (RecommendFollowsResponse, error) {
	var nodeRanking nodeRanking
	var err error

	switch args.Sort {
	case Global:
		nodeRanking, err = s.recommendGlobal(ctx, args)

	case Personalized:
		nodeRanking, err = s.recommendPersonalized(ctx, args)

	case Followers:
		nodeRanking, err = s.recommendFollowers(ctx, args)

	default:
		err = ErrInvalidSort
	}

	if err != nil {
		return nil, fmt.Errorf("failed to recommend with %s: %w", args.Sort, err)
	}

	nodes, ranks := nodeRanking.MaxK(args.Limit).Unpack()
	pubkeys, err := s.redis.Pubkeys(ctx, nodes...)
	if err != nil {
		return nil, err
	}

	response := make(RecommendFollowsResponse, len(pubkeys))
	for i := range pubkeys {
		response[i].Pubkey = pubkeys[i]
		response[i].Rank = ranks[i]
	}
	return response, nil
}

func (s *Service) recommendGlobal(ctx context.Context, args RecommendFollowsArgs) (nodeRanking, error) {
	avoid := make([]graph.ID, 0, 100) // pre-allocate

	node, err := s.redis.NodeByKey(ctx, args.Source)
	if err != nil && !errors.Is(err, graph.ErrNodeNotFound) {
		return nil, err
	}

	if !errors.Is(err, graph.ErrNodeNotFound) {
		// add the node representing the source and its follows to avoid,
		// so they can't get recommended.
		follows, err := s.redis.Follows(ctx, node.ID)
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
	ranks, err := pagerank.Global(ctx, s.redis, candidates...)
	if err != nil {
		return nil, err
	}

	return slicex.Pack(candidates, ranks), nil
}

func (s *Service) recommendFollowers(ctx context.Context, args RecommendFollowsArgs) (nodeRanking, error) {
	avoid := make([]graph.ID, 0, 100) // pre-allocate

	node, err := s.redis.NodeByKey(ctx, args.Source)
	if err != nil && !errors.Is(err, graph.ErrNodeNotFound) {
		return nil, err
	}

	if !errors.Is(err, graph.ErrNodeNotFound) {
		// add the node representing the source and its follows to avoid,
		// so they can't get recommended.
		follows, err := s.redis.Follows(ctx, node.ID)
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
	counts, err := s.redis.FollowerCounts(ctx, candidates...)
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

func (s *Service) recommendPersonalized(ctx context.Context, args RecommendFollowsArgs) (nodeRanking, error) {
	source, err := s.redis.NodeByKey(ctx, args.Source)
	if err != nil {
		return nil, err
	}

	follows, err := s.redis.Follows(ctx, source.ID)
	if err != nil {
		return nil, err
	}

	pp, err := pagerank.Personalized(ctx, s.redis, source.ID, 100_000)
	if err != nil {
		return nil, err
	}

	// remove follows and self from the recommendations
	avoid := append(follows, source.ID)
	for _, ID := range avoid {
		delete(pp, ID)
	}

	return slicex.ToPairs(pp), nil
}
