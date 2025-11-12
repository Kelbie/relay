package service

import (
	"context"
	"errors"

	"github.com/pippellia-btc/slicex"
	"github.com/vertex-lab/crawler_v2/pkg/graph"
)

type (
	ranking     = slicex.Pairs[string, float64]   // a slice of (pubkey, rank)
	nodeRanking = slicex.Pairs[graph.ID, float64] // a slice of (node, rank)
)

// VerifyReputation returns the rank of the target and its highest ranked followers.
// All ranks use the specified [Algorithm].
// For more info read: https://vertexlab.io/docs/services/verify-reputation/
func (s Service) VerifyReputation(ctx context.Context, args VerifyReputationArgs) (VerifyReputationResponse, error) {
	target, err := s.redis.NodeByKey(ctx, args.Target)
	if err != nil {
		if errors.Is(err, graph.ErrNodeNotFound) {
			// target is not found, assume it's a low-reputation key (rank of 0)
			response := VerifyReputationResponse{}
			response.target.Pubkey = args.Target
			return response, nil
		}
		return VerifyReputationResponse{}, err
	}

	followers, err := s.redis.Followers(ctx, target.ID)
	if err != nil {
		return VerifyReputationResponse{}, err
	}

	followCount, err := s.redis.FollowCounts(ctx, target.ID)
	if err != nil {
		return VerifyReputationResponse{}, err
	}

	toRank := append([]graph.ID{target.ID}, followers...)
	ranks, err := s.rank(ctx, toRank, args.Algorithm)
	if err != nil {
		return VerifyReputationResponse{}, err
	}

	followerRanking := slicex.Pack(followers, ranks[1:])
	topFollowers, topRanks := followerRanking.MaxK(args.Limit).Unpack()

	topPubkeys, err := s.redis.Pubkeys(ctx, topFollowers...)
	if err != nil {
		return VerifyReputationResponse{}, err
	}

	response := VerifyReputationResponse{}
	response.target.Pubkey = args.Target
	response.target.Rank = ranks[0]
	response.target.Follows = followCount[0]
	response.target.Followers = len(followers)

	response.followers = make([]followerResponse, len(topPubkeys))
	for i := range topPubkeys {
		response.followers[i].Pubkey = topPubkeys[i]
		response.followers[i].Rank = topRanks[i]
	}
	return response, nil
}
