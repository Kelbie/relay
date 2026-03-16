package core

import (
	"context"
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

	ErrUnsuportedVerifyReputation = errors.New("param must be one between 'target', 'source', 'sort', and 'limit'")
)

type VerifyReputationArgs struct {
	Algorithm
	Target string
	Limit  int
}

func NewVerifyReputationArgs(pubkey string) VerifyReputationArgs {
	return VerifyReputationArgs{
		Algorithm: Algorithm{
			Sort:   Global,
			Source: pubkey,
		},
		Limit: 5,
	}
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

type VerifyReputationResult struct {
	Nodes        int
	Target       DetailedProfile
	TopFollowers []Profile
}

// VerifyReputation returns the rank of the target and its highest ranked followers.
// All ranks use the specified [Algorithm].
// For more info read: https://vertexlab.io/docs/services/verify-reputation/
func (s *Service) VerifyReputation(ctx context.Context, args VerifyReputationArgs) (VerifyReputationResult, error) {
	response, err := s.verifyReputation(ctx, args)
	if err != nil {
		return VerifyReputationResult{}, fmt.Errorf("VerifyReputation %w: %w", ErrInternal, err)
	}
	return response, nil
}

func (s *Service) verifyReputation(ctx context.Context, args VerifyReputationArgs) (VerifyReputationResult, error) {
	nodes, err := s.Graph.NodeCount(ctx)
	if err != nil {
		return VerifyReputationResult{}, err
	}

	target, err := s.Graph.NodeByKey(ctx, args.Target)
	if errors.Is(err, graph.ErrNodeNotFound) {
		// target is not found, assume it's a low-reputation key (rank of 0)
		response := VerifyReputationResult{}
		response.Nodes = nodes
		response.Target.Pubkey = args.Target
		return response, nil
	}
	if err != nil {
		return VerifyReputationResult{}, err
	}

	followers, err := s.Graph.Followers(ctx, target.ID)
	if err != nil {
		return VerifyReputationResult{}, err
	}

	followCount, err := s.Graph.FollowCounts(ctx, target.ID)
	if err != nil {
		return VerifyReputationResult{}, err
	}

	toRank := append([]graph.ID{target.ID}, followers...)
	ranks, err := s.rankNodes(ctx, toRank, args.Algorithm)
	if err != nil {
		return VerifyReputationResult{}, err
	}

	followerRanking := slicex.Pack(followers, ranks[1:])
	topFollowers, topRanks := followerRanking.MaxK(args.Limit).Unpack()

	topPubkeys, err := s.Graph.Pubkeys(ctx, topFollowers...)
	if err != nil {
		return VerifyReputationResult{}, err
	}

	response := VerifyReputationResult{}
	response.Nodes = nodes
	response.Target.Pubkey = args.Target
	response.Target.Rank = ranks[0]
	response.Target.Follows = followCount[0]
	response.Target.Followers = len(followers)
	response.TopFollowers = make([]Profile, len(topPubkeys))

	for i := range topPubkeys {
		response.TopFollowers[i].Pubkey = topPubkeys[i]
		response.TopFollowers[i].Rank = topRanks[i]
	}

	if target.Status == graph.StatusLeaked {
		leakedSecret, leakedAt, err := s.Leaks.Read(ctx, target.Pubkey)
		if err != nil {
			return VerifyReputationResult{}, err
		}
		response.Target.LeakedSecret = leakedSecret
		response.Target.LeakedAt = leakedAt.Unix()
	}
	return response, nil
}
