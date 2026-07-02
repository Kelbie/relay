package ranking

import (
	"context"
	"errors"
	"fmt"
	"slices"

	ore "github.com/Open-Ranking/go-sdk"
	"github.com/vertex-lab/crawler_v2/pkg/graph"
)

var supportedAlgoStats = []ore.AlgorithmID{
	GlobalPagerank, FollowersCount, PersonalizedPagerank,
}

type StatsPubkeyRequest ore.StatsPubkeyRequest

func (r *StatsPubkeyRequest) Normalize() error {
	if err := validatePubkey(r.Pubkey); err != nil {
		return err
	}
	if r.Algorithm == "" {
		r.Algorithm = GlobalPagerank
	}
	if !slices.Contains(supportedAlgoStats, r.Algorithm) {
		return fmt.Errorf("invalid algorithm: %s", r.Algorithm)
	}
	if r.Algorithm == PersonalizedPagerank {
		if err := validatePubkey(r.POV); err != nil {
			return fmt.Errorf("invalid pov: %w", err)
		}
	}
	return nil
}

func (r *StatsPubkeyRequest) Cost() int {
	if r.Algorithm == PersonalizedPagerank {
		return 10
	}
	return 1
}

// StatsPubkey returns the stats for a given pubkey, as defined by ORE-02.
// The request is assumed to have been validated by the caller.
// Learn more here: https://github.com/Open-Ranking/protocol/blob/main/02.md
func (s *Service) StatsPubkey(ctx context.Context, r StatsPubkeyRequest) (ore.StatsPubkeyResponse, error) {
	target, err := s.Graph.NodeByKey(ctx, r.Pubkey)
	if errors.Is(err, graph.ErrNodeNotFound) {
		return ore.StatsPubkeyResponse{}, ErrUnknownPubkey
	}
	if err != nil {
		return ore.StatsPubkeyResponse{}, err
	}

	var rank float64
	if target.Status != graph.StatusLeaked {
		// if a pubkey has been leaked we show its rank as 0, otherwise we rank it.
		ranks, err := s.rankNodes(ctx, r.Algorithm, r.POV, target.ID)
		if err != nil {
			return ore.StatsPubkeyResponse{}, err
		}
		rank = ranks[0]
	}

	follows, err := s.Graph.FollowCounts(ctx, target.ID)
	if err != nil {
		return ore.StatsPubkeyResponse{}, err
	}

	followers, err := s.Graph.FollowerCounts(ctx, target.ID)
	if err != nil {
		return ore.StatsPubkeyResponse{}, err
	}

	// stats of a pubkey change rather frequently, so I prefer to remain
	// conservative and suggest a TTL of just 1 hours.
	ttl := hour

	res := ore.StatsPubkeyResponse{
		Pubkey:    r.Pubkey,
		Rank:      rank,
		Follows:   &follows[0],
		Followers: &followers[0],
		TTL:       &ttl,
	}
	return res, nil
}
