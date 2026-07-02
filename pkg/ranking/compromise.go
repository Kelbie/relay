package ranking

import (
	"context"

	ore "github.com/Open-Ranking/go-sdk"
	"github.com/Open-Ranking/go-sdk/compromise"
)

// CompromisePubkeys returns the compromise information rank of a batch of pubkeys, as defined by ORE-08.
// The request is assumed to have been validated by the caller.
// Learn more here: https://github.com/Open-Ranking/protocol/blob/main/08.md
func (s *Service) CompromisePubkeys(ctx context.Context, r ore.CompromisedPubkeysRequest) (ore.CompromisedPubkeysResponse, error) {
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
