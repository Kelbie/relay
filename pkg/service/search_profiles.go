package service

import (
	"fmt"
	"slices"
	"strings"

	"github.com/nbd-wtf/go-nostr"
)

var (
	SearchProfilesSorts = []string{Global, Personalized, Followers}
	SearchProfilesLimit = 100
)

type SearchProfilesArgs struct {
	Algorithm
	Search string
	Limit  int
}

// Normalize the args in place. It validates all the arguments, converting from
// npub to hex pubkeys if necessary.
func (a *SearchProfilesArgs) Normalize() error {
	if a.Limit < 1 || a.Limit > SearchProfilesLimit {
		return fmt.Errorf("%w: limit must be between 1 and %d: %d", ErrInvalidLimit, SearchProfilesLimit, a.Limit)
	}

	if !slices.Contains(SearchProfilesSorts, a.Sort) {
		return fmt.Errorf("%w: sort must be one between %v: %v", ErrInvalidSort, SearchProfilesSorts, a.Sort)
	}

	if a.Sort == Personalized && !nostr.IsValidPublicKey(a.Source) {
		var err error
		a.Source, err = NpubToHex(a.Source)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidSource, err)
		}
	}

	a.Search = strings.TrimSpace(a.Search)
	if len(a.Search) < 3 {
		return fmt.Errorf("%w: the search parameter must be longer than 3 characters (excluding leading and trailing spaces)", ErrInvalidSearch)
	}
	return nil
}

// Cost returns the cost (measured in credits) of a service call with the provided arguments.
func (a SearchProfilesArgs) Cost() int {
	if a.Sort == Personalized {
		return 10
	}
	return 1
}

type SearchProfilesItem struct {
	Pubkey string  `json:"pubkey"`
	Rank   float64 `json:"rank"`
}

type SearchProfilesResponse []RankProfilesItem
