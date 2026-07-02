package ranking

import (
	"context"
	"fmt"

	ore "github.com/Open-Ranking/go-sdk"
	"github.com/pippellia-btc/slicex"
	"github.com/vertex-lab/crawler_v2/pkg/graph"
	"github.com/vertex-lab/crawler_v2/pkg/pagerank"
)

// RankPubkeys returns the rank of a batch of pubkeys, as defined by ORE-03.
// The request is assumed to have been validated by the caller.
// Learn more here: https://github.com/Open-Ranking/protocol/blob/main/03.md
func (s *Service) RankPubkeys(ctx context.Context, r ore.RankPubkeysRequest) (ore.RankPubkeysResponse, error) {
	nodes, err := s.Graph.NodeIDs(ctx, r.Pubkeys...)
	if err != nil {
		return ore.RankPubkeysResponse{}, err
	}
	ranks, err := s.rankNodes(ctx, r.Algorithm, r.POV, nodes...)
	if err != nil {
		return ore.RankPubkeysResponse{}, err
	}

	ranking := slicex.Pack(r.Pubkeys, ranks)
	top := ranking.MaxK(r.Limit)
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
