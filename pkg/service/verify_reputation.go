package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/slicex"
	"github.com/vertex-lab/crawler_v2/pkg/graph"
)

var (
	VerifyReputationSorts = []string{Global, Personalized, Followers}
	VerifyReputationLimit = 100
)

type VerifyReputationArgs struct {
	Algorithm
	Target string
	Limit  int
}

// Normalize the args in place. It validates all the arguments, converting from
// npub to hex pubkeys if necessary.
func (a *VerifyReputationArgs) Normalize() error {
	if a.Limit < 1 || a.Limit > VerifyReputationLimit {
		return fmt.Errorf("%w: limit must be an integer between 1 and %d: %d", ErrInvalidLimit, VerifyReputationLimit, a.Limit)
	}

	if !slices.Contains(VerifyReputationSorts, a.Sort) {
		return fmt.Errorf("%w: sort must be one between %v: %v", ErrInvalidSort, VerifyReputationSorts, a.Sort)
	}

	if a.Sort == Personalized && !nostr.IsValidPublicKey(a.Source) {
		var err error
		a.Source, err = NpubToHex(a.Source)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidSource, err)
		}
	}

	if !nostr.IsValidPublicKey(a.Target) {
		var err error
		a.Target, err = NpubToHex(a.Target)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidTarget, err)
		}
	}
	return nil
}

// Cost returns the cost (measured in credits) of a service call with the provided arguments.
func (a VerifyReputationArgs) Cost() int {
	if a.Sort == Personalized {
		return 10
	}
	return 1
}

type VerifyReputationResponse struct {
	target    targetResponse
	followers []followerResponse
}

type targetResponse struct {
	Pubkey    string  `json:"pubkey"`
	Rank      float64 `json:"rank"`
	Follows   int     `json:"follows"`
	Followers int     `json:"followers"`
}

type followerResponse struct {
	Pubkey string  `json:"pubkey"`
	Rank   float64 `json:"rank"`
}

func (r VerifyReputationResponse) MarshalJSON() ([]byte, error) {
	array := make([]any, 0, len(r.followers)+1)
	array = append(array, r.target)
	for _, f := range r.followers {
		array = append(array, f)
	}
	return json.Marshal(array)
}

// VerifyReputation returns the rank of the target and its highest ranked followers.
// All ranks use the specified [Algorithm].
// For more info read: https://vertexlab.io/docs/services/verify-reputation/
func (s *Service) VerifyReputation(ctx context.Context, args VerifyReputationArgs) (VerifyReputationResponse, error) {
	response, err := s.verifyReputation(ctx, args)
	if err != nil {
		return VerifyReputationResponse{}, fmt.Errorf("VerifyReputation %w: %w", ErrInternal, err)
	}
	return response, nil
}

func (s *Service) verifyReputation(ctx context.Context, args VerifyReputationArgs) (VerifyReputationResponse, error) {
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
