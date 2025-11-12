package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/slicex"
	"github.com/vertex-lab/crawler_v2/pkg/graph"
)

type (
	ranking     = slicex.Pairs[string, float64]   // a slice of (pubkey, rank)
	nodeRanking = slicex.Pairs[graph.ID, float64] // a slice of (node, rank)
)

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

// SearchProfiles returns the top ranked pubkeys whose kind:0s contain the provided string.
// All ranks use the specified [Algorithm].
// For more info read: https://vertexlab.io/docs/services/search-profiles/
func (s *Service) SearchProfiles(ctx context.Context, args SearchProfilesArgs) (SearchProfilesResponse, error) {
	response, err := s.searchProfiles(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("SearchProfiles %w: %w", ErrInternal, err)
	}
	return response, nil
}

func (s *Service) searchProfiles(ctx context.Context, args SearchProfilesArgs) (SearchProfilesResponse, error) {
	if nostr.IsValidPublicKey(args.Search) {
		return SearchProfilesResponse{{Pubkey: args.Search, Rank: 1}}, nil
	}

	if strings.HasPrefix(args.Search, "npub") {
		pk, err := NpubToHex(args.Search)
		if err == nil {
			// decode it to hex and return only if it's a valid npub.
			// otherwise, continue with the full text search.
			return SearchProfilesResponse{{Pubkey: pk, Rank: 1}}, nil
		}
	}

	ranking, err := s.search(ctx, args.Search)
	if err != nil {
		return nil, err
	}

	pubkeys, searchRanks := ranking.Unpack()
	nodes, err := s.redis.NodeIDs(ctx, pubkeys...)
	if err != nil {
		return nil, err
	}

	reputations, err := s.rank(ctx, nodes, args.Algorithm)
	if err != nil {
		return nil, err
	}

	for i := range ranking {
		// merge reputational and search ranks
		ranking[i].Val = math.Pow(searchRanks[i], 3) * reputations[i]
	}

	topPubkeys, topRanks := ranking.MaxK(args.Limit).Unpack()
	response := make(SearchProfilesResponse, len(topPubkeys))

	for i := range topPubkeys {
		response[i].Pubkey = topPubkeys[i]
		response[i].Rank = topRanks[i]
	}
	return response, nil
}

// Search performs full text seach on the profiles (kind:0s) using the specified search term.
// It returns the pubkeys and search scores (positives, higher is better) of the SQL query.
func (s *Service) search(ctx context.Context, search string) (ranking, error) {
	search = escapeFTS5(search)
	row := s.sqlite.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM profiles_fts WHERE profiles_fts MATCH ?", search)

	var matches int
	if err := row.Scan(&matches); err != nil {
		return nil, fmt.Errorf("failed to count matches: %w", err)
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
		return nil, fmt.Errorf("failed to make full text search: %w", err)
	}
	defer rows.Close()

	ranking := make(ranking, 0, limit) // pre-allocating
	for rows.Next() {
		var pair slicex.Pair[string, float64]
		if err = rows.Scan(&pair.Key, &pair.Val); err != nil {
			return nil, fmt.Errorf("failed to scan the results of the query: %w", err)
		}

		// convert bm25 scores (negative) to positive to have best is highest
		pair.Val = -pair.Val
		ranking = append(ranking, pair)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan the results of the query: %w", err)
	}
	return ranking, nil
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
