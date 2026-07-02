package ranking

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"

	ore "github.com/Open-Ranking/go-sdk"
	"github.com/pippellia-btc/slicex"
	"github.com/vertex-lab/crawler_v2/pkg/graph"
	"github.com/vertex-lab/crawler_v2/pkg/pagerank"
)

var supportedAlgoRecommend = []ore.AlgorithmID{
	GlobalPagerank, FollowersCount, PersonalizedPagerank,
}

type RecommendPubkeysRequest ore.RecommendPubkeysRequest

func (r *RecommendPubkeysRequest) Normalize() error {
	if r.Limit == 0 {
		r.Limit = 20
	}
	if r.Limit < 0 || r.Limit > 100 {
		return fmt.Errorf("invalid limit: %d", r.Limit)
	}

	if r.Algorithm == "" {
		r.Algorithm = GlobalPagerank
	}
	if !slices.Contains(supportedAlgoRecommend, r.Algorithm) {
		return fmt.Errorf("invalid algorithm: %s", r.Algorithm)
	}
	if r.Algorithm == PersonalizedPagerank {
		if err := validatePubkey(r.POV); err != nil {
			return fmt.Errorf("invalid pov: %w", err)
		}
	}
	return nil
}

func (r *RecommendPubkeysRequest) Cost() int {
	if r.Algorithm == PersonalizedPagerank {
		return 10
	}
	return 1
}

// RecommendPubkeys returns a batch of pubkeys that are recommended, as defined by ORE-04.
// The request is assumed to have been validated by the caller.
// Learn more here: https://github.com/Open-Ranking/protocol/blob/main/04.md
func (s *Service) RecommendPubkeys(ctx context.Context, r RecommendPubkeysRequest) (ore.RecommendPubkeysResponse, error) {
	candidates, err := s.candidates(ctx, r.Algorithm, r.POV)
	if err != nil {
		return ore.RecommendPubkeysResponse{}, err
	}

	nodes, ranks := candidates.MaxK(r.Limit).Unpack()
	pubkeys, err := s.Graph.Pubkeys(ctx, nodes...)
	if err != nil {
		return ore.RecommendPubkeysResponse{}, err
	}

	ttl := TTL(r.Algorithm)
	res := ore.RecommendPubkeysResponse{
		Results: make([]ore.RankedPubkey, len(pubkeys)),
		TTL:     &ttl,
	}
	for i := range pubkeys {
		res.Results[i].Pubkey = pubkeys[i]
		res.Results[i].Rank = ranks[i]
	}
	return res, nil
}

// nodeRanking is a slice of pairs (nodeID, rank)
type nodeRanking = slicex.Pairs[graph.ID, float64]

func (s *Service) candidates(ctx context.Context, algo ore.AlgorithmID, pov string) (nodeRanking, error) {
	switch algo {
	case GlobalPagerank:
		return s.candidatesGlobal(ctx, pov)
	case PersonalizedPagerank:
		return s.candidatesPersonalized(ctx, pov)
	case FollowersCount:
		return s.candidatesFollowers(ctx, pov)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlgo, algo)
	}
}

func (s *Service) candidatesGlobal(ctx context.Context, source string) (nodeRanking, error) {
	avoid := make([]graph.ID, 0, 100) // pre-allocate

	node, err := s.Graph.NodeByKey(ctx, source)
	if err != nil && !errors.Is(err, graph.ErrNodeNotFound) {
		return nil, err
	}

	if !errors.Is(err, graph.ErrNodeNotFound) {
		// add the node representing the source and its follows to avoid,
		// so they can't get recommended.
		follows, err := s.Graph.Follows(ctx, node.ID)
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
	ranks, err := pagerank.Global(ctx, s.Graph, candidates...)
	if err != nil {
		return nil, err
	}
	return slicex.Pack(candidates, ranks), nil
}

func (s *Service) candidatesFollowers(ctx context.Context, source string) (nodeRanking, error) {
	avoid := make([]graph.ID, 0, 100) // pre-allocate

	node, err := s.Graph.NodeByKey(ctx, source)
	if err != nil && !errors.Is(err, graph.ErrNodeNotFound) {
		return nil, err
	}

	if !errors.Is(err, graph.ErrNodeNotFound) {
		// add the node representing the source and its follows to avoid,
		// so they can't get recommended.
		follows, err := s.Graph.Follows(ctx, node.ID)
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
	counts, err := s.Graph.FollowerCounts(ctx, candidates...)
	if err != nil {
		return nil, err
	}

	ranking := make(nodeRanking, len(candidates))
	for i := range candidates {
		ranking[i].Key = candidates[i]
		ranking[i].Val = float64(counts[i])
	}
	return ranking, nil
}

func (s *Service) candidatesPersonalized(ctx context.Context, source string) (nodeRanking, error) {
	node, err := s.Graph.NodeByKey(ctx, source)
	if err != nil {
		return nil, err
	}

	follows, err := s.Graph.Follows(ctx, node.ID)
	if err != nil {
		return nil, err
	}

	pp, err := pagerank.Personalized(ctx, s.Graph, node.ID, 100_000)
	if err != nil {
		return nil, err
	}

	delete(pp, node.ID)
	for _, ID := range follows {
		delete(pp, ID)
	}
	return slicex.ToPairs(pp), nil
}
