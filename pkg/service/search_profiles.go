package service

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/slicex"
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
	if len(a.Search) < 3 || len(a.Search) > 100 {
		return fmt.Errorf("%w: the search parameter must between 3 and 100 characters (excluding leading and trailing spaces)", ErrInvalidSearch)
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

type SearchProfilesResponse struct {
	Nodes   int
	Results []Profile
}

// SearchProfiles returns the top ranked pubkeys whose kind:0s contain the provided string.
// All ranks use the specified [Algorithm].
// For more info read: https://vertexlab.io/docs/services/search-profiles/
func (s *Service) SearchProfiles(ctx context.Context, args SearchProfilesArgs) (SearchProfilesResponse, error) {
	response, err := s.searchProfiles(ctx, args)
	if err != nil {
		return SearchProfilesResponse{}, fmt.Errorf("SearchProfiles %w: %w", ErrInternal, err)
	}
	return response, nil
}

func (s *Service) searchProfiles(ctx context.Context, args SearchProfilesArgs) (SearchProfilesResponse, error) {
	nodes, err := s.redis.NodeCount(ctx)
	if err != nil {
		return SearchProfilesResponse{}, err
	}

	if nostr.IsValidPublicKey(args.Search) {
		response := SearchProfilesResponse{}
		response.Nodes = nodes
		response.Results = []Profile{{Pubkey: args.Search, Rank: 1}}
		return response, nil
	}

	if strings.HasPrefix(args.Search, "npub") {
		pk, err := NpubToHex(args.Search)
		if err == nil {
			// decode it to hex and return only if it's a valid npub.
			// otherwise, continue with the full text search.
			response := SearchProfilesResponse{}
			response.Nodes = nodes
			response.Results = []Profile{{Pubkey: pk, Rank: 1}}
			return response, nil
		}
	}

	pubkeys, searchRanks, err := s.search(ctx, args.Search)
	if err != nil {
		return SearchProfilesResponse{}, err
	}

	ranks, err := s.rankPubkeys(ctx, pubkeys, args.Algorithm)
	if err != nil {
		return SearchProfilesResponse{}, err
	}

	combinedRanks := make([]float64, len(ranks))
	for i := range ranks {
		combinedRanks[i] = math.Pow(searchRanks[i], 3) * ranks[i]
	}

	ranking := slicex.Pack(pubkeys, combinedRanks)
	topPubkeys, topRanks := ranking.MaxK(args.Limit).Unpack()

	response := SearchProfilesResponse{}
	response.Nodes = nodes
	response.Results = make([]Profile, len(topPubkeys))

	for i := range topPubkeys {
		response.Results[i].Pubkey = topPubkeys[i]
		response.Results[i].Rank = topRanks[i]
	}
	return response, nil
}

// Search performs full text seach on the profiles (kind:0s) using the specified search term.
// It returns the pubkeys and search scores (positives, higher is better) of the SQL query.
func (s *Service) search(ctx context.Context, search string) (pubkeys []string, scores []float64, err error) {
	search = escapeFTS5(search)
	row := s.sqlite.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM profiles_fts WHERE profiles_fts MATCH ?", search)

	var matches int
	if err := row.Scan(&matches); err != nil {
		return nil, nil, fmt.Errorf("failed to count matches: %w", err)
	}

	d, limit := dampening(matches), min(matches, maxSearchLimit)
	name, displayName, about, website, nip05 := 10, 12, 1*d, 1*d, 3*d

	query := `SELECT pubkey, bm25(profiles_fts, 0.0, 0.0, ?, ?, ?, ?, ?) AS score
				FROM profiles_fts
				WHERE profiles_fts MATCH ? AND score < 0
				ORDER BY score
				LIMIT ?;`

	rows, err := s.sqlite.DB.QueryContext(ctx, query, name, displayName, about, website, nip05, search, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make full text search: %w", err)
	}
	defer rows.Close()

	pubkeys = make([]string, 0, limit)
	scores = make([]float64, 0, limit)

	for rows.Next() {
		var pubkey string
		var score float64

		if err = rows.Scan(&pubkey, &score); err != nil {
			return nil, nil, fmt.Errorf("failed to scan the results of the query: %w", err)
		}

		pubkeys = append(pubkeys, pubkey)
		scores = append(scores, -score) // convert bm25 scores (negative) to positive to have best is highest
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("failed to scan the results of the query: %w", err)
	}
	return pubkeys, scores, nil
}

const (
	defaultSearchLimit = 700  // default LIMIT for the full text search query
	maxSearchLimit     = 3000 // maximum LIMIT for the full text search query
)

// escapeFTS5 prepares a search term for SQLite FTS5
func escapeFTS5(term string) string {
	term = strings.ReplaceAll(term, `"`, `""`)
	return `"` + term + `"`
}

// This function returns the dampening coefficient used to decrease the importance of the
// 'about', 'website', 'nip05' columns when performing full-text search.

// The rationale is the following: the higher the 'matches', the lower the weight of such columns.
// When matches surpasses [defaultSearchLimit] (the budget of the query), the coefficient goes to 0.
// This behaviour is useful for searches involving popular nip05/lightning providers (e.g. 'primal', 'alby'),
// or common terms like 'bitcoin' and 'nostr'.
func dampening(matches int) float64 {
	m, l := float64(matches), float64(defaultSearchLimit)
	return math.Max(1-math.Pow(m/l, 2), 0)
}
