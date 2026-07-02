package ranking

import (
	"context"
	"fmt"
	"slices"

	ore "github.com/Open-Ranking/go-sdk"
	"github.com/pippellia-btc/slicex"
	"github.com/vertex-lab/crawler_v2/pkg/graph"
	"github.com/vertex-lab/crawler_v2/pkg/pagerank"
)

var supportedAlgoRank = []ore.AlgorithmID{
	GlobalPagerank, FollowersCount, PersonalizedPagerank,
}

type RankPubkeysRequest ore.RankPubkeysRequest

func (r *RankPubkeysRequest) Normalize() error {
	r.Pubkeys = slicex.Unique(r.Pubkeys)
	if len(r.Pubkeys) == 0 {
		return fmt.Errorf("pubkeys is required")
	}
	if len(r.Pubkeys) > 1000 {
		return fmt.Errorf("too many pubkeys: %d", len(r.Pubkeys))
	}

	if r.Limit < 0 {
		return fmt.Errorf("invalid limit: %d", r.Limit)
	}
	if r.Limit == 0 {
		r.Limit = len(r.Pubkeys)
	}
	r.Limit = min(r.Limit, len(r.Pubkeys))

	if r.Algorithm == "" {
		r.Algorithm = GlobalPagerank
	}
	if !slices.Contains(supportedAlgoRank, r.Algorithm) {
		return fmt.Errorf("invalid algorithm: %s", r.Algorithm)
	}
	if r.Algorithm == PersonalizedPagerank {
		if err := validatePubkey(r.POV); err != nil {
			return fmt.Errorf("invalid pov: %w", err)
		}
	}
	return nil
}

func (r *RankPubkeysRequest) Cost() int {
	if r.Algorithm == PersonalizedPagerank {
		return 10
	}
	return 1
}

// RankPubkeys returns the rank of a batch of pubkeys, as defined by ORE-03.
// The request is assumed to have been validated by the caller.
// Learn more here: https://github.com/Open-Ranking/protocol/blob/main/03.md
func (s *Service) RankPubkeys(ctx context.Context, r RankPubkeysRequest) (ore.RankPubkeysResponse, error) {
	ranks, err := s.rankPubkeys(ctx, r.Algorithm, r.POV, r.Pubkeys...)
	if err != nil {
		return ore.RankPubkeysResponse{}, err
	}

	top := slicex.Pack(r.Pubkeys, ranks).MaxK(r.Limit)
	ttl := TTL(r.Algorithm)

	res := ore.RankPubkeysResponse{
		Results: make([]ore.RankedPubkey, top.Len()),
		TTL:     &ttl,
	}
	for i, t := range top {
		res.Results[i].Pubkey = t.Key
		res.Results[i].Rank = t.Val
	}
	return res, nil
}

// rankPubkeys ranks the pubkeys according to the provided [ore.AlgorithmID].
// If a pubkey is not found, the rank is always assumed to be 0.
func (s *Service) rankPubkeys(ctx context.Context, algo ore.AlgorithmID, pov string, pubkeys ...string) ([]float64, error) {
	if len(pubkeys) == 0 {
		return nil, nil
	}
	nodes, err := s.Graph.NodeIDs(ctx, pubkeys...)
	if err != nil {
		return nil, err
	}
	return s.rankNodes(ctx, algo, pov, nodes...)
}

// rankNodes ranks the nodes according to the provided [ore.AlgorithmID].
// If a node is not found, the rank is always assumed to be 0.
func (s *Service) rankNodes(ctx context.Context, algo ore.AlgorithmID, pov string, nodes ...graph.ID) ([]float64, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	switch algo {
	case FollowersCount:
		counts, err := s.Graph.FollowerCounts(ctx, nodes...)
		if err != nil {
			return nil, err
		}

		ranks := make([]float64, len(counts))
		for i, count := range counts {
			ranks[i] = float64(count)
		}
		return ranks, nil

	case GlobalPagerank:
		return pagerank.Global(ctx, s.Graph, nodes...)

	case PersonalizedPagerank:
		source, err := s.Graph.NodeByKey(ctx, pov)
		if err != nil {
			return nil, err
		}
		return pagerank.PersonalizedWithTargets(ctx, s.Graph, source.ID, nodes, 100_000)

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlgo, algo)
	}
}
