package ranking

import (
	"context"
	"fmt"
	"slices"

	ore "github.com/Open-Ranking/go-sdk"
	"github.com/Open-Ranking/go-sdk/compromise"
	"github.com/pippellia-btc/slicex"
)

var supportedAlgoCompromised = []ore.AlgorithmID{
	SignatureProof,
}

type CompromisedPubkeysRequest ore.CompromisedPubkeysRequest

func (r *CompromisedPubkeysRequest) Normalize() error {
	r.Pubkeys = slicex.Unique(r.Pubkeys)
	if len(r.Pubkeys) == 0 {
		return fmt.Errorf("pubkeys is required")
	}
	if len(r.Pubkeys) > 100 {
		return fmt.Errorf("too many pubkeys: %d", len(r.Pubkeys))
	}
	if r.Algorithm == "" {
		r.Algorithm = SignatureProof
	}
	if !slices.Contains(supportedAlgoCompromised, r.Algorithm) {
		return fmt.Errorf("invalid algorithm: %s", r.Algorithm)
	}
	return nil
}

func (r *CompromisedPubkeysRequest) Cost() int {
	return 1
}

// CompromisedPubkeys returns the compromise information rank of a batch of pubkeys, as defined by ORE-08.
// The request is assumed to have been validated by the caller.
// Learn more here: https://github.com/Open-Ranking/protocol/blob/main/08.md
func (s *Service) CompromisedPubkeys(ctx context.Context, r CompromisedPubkeysRequest) (ore.CompromisedPubkeysResponse, error) {
	records, err := s.Leaks.ReadMany(ctx, r.Pubkeys...)
	if err != nil {
		return ore.CompromisedPubkeysResponse{}, err
	}

	res := make(ore.CompromisedPubkeysResponse, len(records))
	for _, rec := range records {
		detectedAt := rec.DetectedAt.Unix()
		res[rec.Pubkey] = compromise.Confirmed{
			Proof:      rec.Proof,
			DetectedAt: &detectedAt,
		}
	}
	return res, nil
}
