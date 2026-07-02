package ranking

import (
	"context"
	"errors"
	"fmt"
	"slices"

	ore "github.com/Open-Ranking/go-sdk"
	"github.com/pippellia-btc/slicex"
	"github.com/vertex-lab/crawler_v2/pkg/graph"
)

var supportedAlgoFollowers = []ore.AlgorithmID{
	GlobalPagerank, FollowersCount, PersonalizedPagerank,
}

type FollowersRequest ore.FollowersRequest

func (r *FollowersRequest) Normalize() error {
	if r.Limit == 0 {
		r.Limit = 50
	}
	if r.Limit < 0 || r.Limit > 1000 {
		return fmt.Errorf("invalid limit: %d", r.Limit)
	}

	if err := validatePubkey(r.Pubkey); err != nil {
		return err
	}
	if r.Algorithm == "" {
		r.Algorithm = GlobalPagerank
	}
	if !slices.Contains(supportedAlgoFollowers, r.Algorithm) {
		return fmt.Errorf("invalid algorithm: %s", r.Algorithm)
	}
	if r.Algorithm == PersonalizedPagerank {
		if err := validatePubkey(r.POV); err != nil {
			return fmt.Errorf("invalid pov: %w", err)
		}
	}
	return nil
}

func (r *FollowersRequest) Cost() int {
	if r.Algorithm == PersonalizedPagerank {
		return 10
	}
	return 1
}

// Followers returns the top followers of the specified pubkey as defined by ORE-06.
// The request is assumed to have been validated by the caller.
// Learn more here: https://github.com/Open-Ranking/protocol/blob/main/06.md
func (s *Service) Followers(ctx context.Context, r FollowersRequest) (ore.FollowersResponse, error) {
	target, err := s.Graph.NodeByKey(ctx, r.Pubkey)
	if errors.Is(err, graph.ErrNodeNotFound) {
		return ore.FollowersResponse{}, ErrUnknownPubkey
	}
	if err != nil {
		return ore.FollowersResponse{}, err
	}

	// TODO: fetching all followers is expensive for large accounts; cap or paginate.
	followers, err := s.Graph.Followers(ctx, target.ID)
	if err != nil {
		return ore.FollowersResponse{}, err
	}

	ranks, err := s.rankNodes(ctx, r.Algorithm, r.POV, followers...)
	if err != nil {
		return ore.FollowersResponse{}, err
	}

	topFollowers, topRanks := slicex.Pack(followers, ranks).MaxK(r.Limit).Unpack()
	topPubkeys, err := s.Graph.Pubkeys(ctx, topFollowers...)
	if err != nil {
		return ore.FollowersResponse{}, err
	}

	ttl := TTL(r.Algorithm)
	total := len(followers)
	res := ore.FollowersResponse{
		Results: make([]ore.RankedPubkey, len(topFollowers)),
		Total:   &total,
		TTL:     &ttl,
	}
	for i := range topPubkeys {
		res.Results[i].Pubkey = topPubkeys[i]
		res.Results[i].Rank = topRanks[i]
	}
	return res, nil
}
