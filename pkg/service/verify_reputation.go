package service

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/nbd-wtf/go-nostr"
)

var (
	VR_ValidSorts = []string{Global, Personalized, Followers}
	VR_MaxLimit   = 100
)

type VerifyReputationArgs struct {
	Algorithm
	Target string
	Limit  int
}

// Normalize the args in place. It validates all the arguments, converting from
// npub to hex pubkeys if necessary.
func (a *VerifyReputationArgs) Normalize() error {
	if a.Limit < 1 || a.Limit > VR_MaxLimit {
		return fmt.Errorf("%w: limit must be an integer between 1 and %d: %d", ErrInvalidLimit, VR_MaxLimit, a.Limit)
	}

	if !slices.Contains(VR_ValidSorts, a.Sort) {
		return fmt.Errorf("%w: sort must be one between %v: %v", ErrInvalidSort, VR_ValidSorts, a.Sort)
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
