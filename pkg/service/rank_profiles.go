package service

import (
	"context"
	"fmt"
	"slices"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/slicex"
)

var (
	RankProfilesSorts = []string{Global, Personalized, Followers}
	RankProfilesLimit = 1000
)

type RankProfilesArgs struct {
	Algorithm
	Targets []string
	Limit   int
}

// Normalize the args in place. It validates all the arguments, converting from
// npub to hex pubkeys if necessary.
func (a *RankProfilesArgs) Normalize() error {
	if a.Limit < 1 || a.Limit > RankProfilesLimit {
		return fmt.Errorf("%w: limit must be between 1 and %d: %d", ErrInvalidLimit, RankProfilesLimit, a.Limit)
	}

	if !slices.Contains(RankProfilesSorts, a.Sort) {
		return fmt.Errorf("%w: sort must be one between %v: %v", ErrInvalidSort, RankProfilesSorts, a.Sort)
	}

	if a.Sort == Personalized && !nostr.IsValidPublicKey(a.Source) {
		var err error
		a.Source, err = NpubToHex(a.Source)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidSource, err)
		}
	}

	a.Targets = slicex.Unique(a.Targets)
	if len(a.Targets) < 1 || len(a.Targets) > RankProfilesLimit {
		return fmt.Errorf("%w: the number of targets must be between 1 and %d: %d", ErrInvalidTargets, RankProfilesLimit, len(a.Targets))
	}

	for i, target := range a.Targets {
		if !nostr.IsValidPublicKey(target) {
			pk, err := NpubToHex(target)
			if err != nil {
				// do nothing on invalid pubkeys; they will simply receive a rank of 0.
				continue
			}

			a.Targets[i] = pk
		}
	}

	a.Limit = min(a.Limit, len(a.Targets))
	return nil
}

// Cost returns the cost (measured in credits) of a service call with the provided arguments.
func (a RankProfilesArgs) Cost() int {
	if a.Sort == Personalized {
		return 10
	}
	return 1
}

type RankProfilesItem struct {
	Pubkey string  `json:"pubkey"`
	Rank   float64 `json:"rank"`
}

type RankProfilesResponse []RankProfilesItem

// RankProfiles returns the rank of the top "limit" targets.
// All ranks use the specified [Algorithm].
// For more info read: https://vertexlab.io/docs/services/sort-profiles/
func (s *Service) RankProfiles(ctx context.Context, args RankProfilesArgs) (RankProfilesResponse, error) {
	response, err := s.rankProfiles(ctx, args)
	if err != nil {
		return RankProfilesResponse{}, fmt.Errorf("RankProfiles %w: %w", ErrInternal, err)
	}
	return response, nil
}

func (s *Service) rankProfiles(ctx context.Context, args RankProfilesArgs) (RankProfilesResponse, error) {
	targets, err := s.redis.NodeIDs(ctx, args.Targets...)
	if err != nil {
		return nil, err
	}

	ranks, err := s.rank(ctx, targets, args.Algorithm)
	if err != nil {
		return nil, err
	}

	ranking := slicex.Pack(args.Targets, ranks)
	topTargets, topRanks := ranking.MaxK(args.Limit).Unpack()

	response := make(RankProfilesResponse, len(topTargets))
	for i := range topTargets {
		response[i].Pubkey = topTargets[i]
		response[i].Rank = topRanks[i]
	}
	return response, nil
}
