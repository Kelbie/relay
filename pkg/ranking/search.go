package ranking

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"

	ore "github.com/Open-Ranking/go-sdk"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/pippellia-btc/slicex"
)

var supportedAlgoSearch = []ore.AlgorithmID{
	GlobalPagerank, FollowersCount, PersonalizedPagerank,
}

type SearchPubkeysRequest ore.SearchPubkeysRequest

func (r *SearchPubkeysRequest) Normalize() error {
	if r.Limit == 0 {
		r.Limit = 10
	}
	if r.Limit < 0 || r.Limit > 100 {
		return fmt.Errorf("invalid limit: %d", r.Limit)
	}

	if r.Algorithm == "" {
		r.Algorithm = GlobalPagerank
	}
	if !slices.Contains(supportedAlgoSearch, r.Algorithm) {
		return fmt.Errorf("invalid algorithm: %s", r.Algorithm)
	}
	if r.Algorithm == PersonalizedPagerank {
		if err := validatePubkey(r.POV); err != nil {
			return fmt.Errorf("invalid pov: %w", err)
		}
	}

	r.Query = strings.TrimSpace(r.Query)
	if len(r.Query) < 3 || len(r.Query) > 100 {
		return fmt.Errorf("invalid search: the search parameter must between 3 and 100 characters")
	}
	return nil
}

func (r *SearchPubkeysRequest) Cost() int {
	if r.Algorithm == PersonalizedPagerank {
		return 10
	}
	return 1
}

// SearchPubkeys returns the pubkeys that match a search query, as defined by ORE-05.
// The request is assumed to have been validated by the caller.
// Learn more here: https://github.com/Open-Ranking/protocol/blob/main/05.md
func (s *Service) SearchPubkeys(ctx context.Context, r SearchPubkeysRequest) (ore.SearchPubkeysResponse, error) {
	if nostr.IsValidPublicKey(r.Query) {
		ttl := infinite
		res := ore.SearchPubkeysResponse{
			Results: []ore.RankedPubkey{{Pubkey: r.Query, Rank: 1}},
			TTL:     &ttl,
		}
		return res, nil
	}

	if strings.HasPrefix(r.Query, "npub1") {
		pk, err := npubToHex(r.Query)
		if err == nil {
			// decode it to hex and return only if it's a valid npub.
			// otherwise, continue with the full text search.
			ttl := infinite
			res := ore.SearchPubkeysResponse{
				Results: []ore.RankedPubkey{{Pubkey: pk, Rank: 1}},
				TTL:     &ttl,
			}
			return res, nil
		}
	}

	pubkeys, searchRanks, err := s.searchPubkeys(ctx, r.Query)
	if err != nil {
		return ore.SearchPubkeysResponse{}, err
	}
	if len(pubkeys) == 0 {
		return ore.SearchPubkeysResponse{Results: []ore.RankedPubkey{}}, nil
	}

	ranks, err := s.rankPubkeys(ctx, r.Algorithm, r.POV, pubkeys...)
	if err != nil {
		return ore.SearchPubkeysResponse{}, err
	}

	combinedRanks := make([]float64, len(ranks))
	for i := range ranks {
		combinedRanks[i] = math.Pow(searchRanks[i], 3) * ranks[i]
	}

	top := slicex.Pack(pubkeys, combinedRanks).MaxK(r.Limit)
	ttl := 5 * minute // search results vary quickly, as people can change their names

	res := ore.SearchPubkeysResponse{
		Results: make([]ore.RankedPubkey, top.Len()),
		TTL:     &ttl,
	}
	for i, t := range top {
		res.Results[i].Pubkey = t.Key
		res.Results[i].Rank = t.Val
	}
	return res, nil
}

// searchPubkeys performs full text seach on the profiles (kind:0s) using the specified search term.
// It returns the pubkeys and search scores (positives, higher is better) of the SQL query.
func (s *Service) searchPubkeys(ctx context.Context, search string) (pubkeys []string, scores []float64, err error) {
	search = escapeFTS5(search)
	row := s.Sqlite.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM profiles_fts WHERE profiles_fts MATCH ?", search)

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

	rows, err := s.Sqlite.DB.QueryContext(ctx, query, name, displayName, about, website, nip05, search, limit)
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

// dampening returns the dampening coefficient used to decrease the importance of the
// 'about', 'website', 'nip05' columns when performing full-text search.
//
// The rationale is the following: the higher the 'matches', the lower the weight of such columns.
// When matches surpasses [defaultSearchLimit] (the budget of the query), the coefficient goes to 0.
// This behaviour is useful for searches involving popular nip05/lightning providers (e.g. 'primal', 'alby'),
// or common terms like 'bitcoin' and 'nostr'.
func dampening(matches int) float64 {
	m, l := float64(matches), float64(defaultSearchLimit)
	return math.Max(1-math.Pow(m/l, 2), 0)
}

// npubToHex tries to convert an npub to an hex pubkey.
func npubToHex(key string) (string, error) {
	key = strings.TrimSpace(key)
	if strings.HasPrefix(key, "npub1") {
		_, pubkey, err := nip19.Decode(key)
		if err != nil {
			return "", fmt.Errorf("%w: '%s'", ErrBadlyFormattedKey, key)
		}

		pk, ok := pubkey.(string)
		if !ok {
			return "", fmt.Errorf("%w: '%s'", ErrBadlyFormattedKey, key)
		}
		return pk, nil
	}
	return "", fmt.Errorf("%w: '%s'", ErrBadlyFormattedKey, key)
}
