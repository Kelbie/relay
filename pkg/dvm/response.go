// The dvm package is responsible for parsing and responding to DVM requests.
package dvm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/slicex"
	"github.com/vertex-lab/crawler_v2/pkg/graph"
	"github.com/vertex-lab/crawler_v2/pkg/pagerank"
	"github.com/vertex-lab/crawler_v2/pkg/redb"
	sqlite "github.com/vertex-lab/nostr-sqlite"
)

var (
	KindVerifyReputation int = 5312
	KindRecommendFollows int = 5313
	KindRankProfiles     int = 5314
	KindSearchProfiles   int = 5315
	KindDVMError         int = 7000
)

type Response []ResponseItem

type ResponseItem struct {
	Pubkey string  `json:"pubkey"`
	Rank   float64 `json:"rank"`
	Extra
}

// Extra groups optional information about a pubkey. Nil means the field is missing.
type Extra struct {
	Follows   *int `json:"follows,omitempty"`
	Followers *int `json:"followers,omitempty"`
}

type (
	ranking     = slicex.Pairs[string, float64]   // a slice of (pubkey, rank)
	nodeRanking = slicex.Pairs[graph.ID, float64] // a slice of (node, rank)
)

// NewResponse combines the ranking and the optional [Extra]s into a [Response].
func NewResponse(ranking ranking, extras ...Extra) Response {
	res := make(Response, len(ranking))
	for i, pair := range ranking {
		res[i] = ResponseItem{Pubkey: pair.Key, Rank: pair.Val}
	}

	for i, extra := range extras {
		res[i].Extra = extra
	}
	return res
}

// Pubkeys returns the pubkeys present in the response.
func (r Response) Pubkeys() []string {
	pubkeys := make([]string, len(r))
	for i, item := range r {
		pubkeys[i] = item.Pubkey
	}
	return pubkeys
}

// ErrorEvent returns an unsigned nostr event for the DVM error
func ErrorEvent(err error, rec Record) *nostr.Event {
	return &nostr.Event{
		CreatedAt: nostr.Now(),
		Kind:      KindDVMError,
		Tags:      append(rec.ToTags(), nostr.Tag{"status", "error", err.Error()}),
	}
}

// ResponseEvent returns an unsigned nostr event used for the DVM.
// The `CreatedAt` field in the response event shows how old the ranking data is.
func ResponseEvent(res Response, req *Request) *nostr.Event {
	json, err := json.Marshal(res)
	if err != nil {
		return ErrorEvent(err, req.Record)
	}

	return &nostr.Event{
		Content:   string(json),
		CreatedAt: req.Timestamp, // shows how old the ranking data is
		Kind:      req.Kind + 1000,
		Tags:      req.ToTags(),
	}
}

// VerifyReputation returns the rank of the target and its highest ranked followers.
// All ranks use the specified [Algorithm].
// For more info read: https://vertexlab.io/docs/services/verify-reputation/
func VerifyReputation(ctx context.Context, db redb.RedisDB, request *Request) (Response, error) {
	args, err := request.ToVerifyReputationArgs()
	if err != nil {
		return nil, err
	}

	ranking, extras, err := verifyReputation(ctx, db, args)
	if err != nil {
		return nil, fmt.Errorf("VerifyReputation %w: %w", ErrInternal, err)
	}
	return NewResponse(ranking, extras...), nil
}

func verifyReputation(ctx context.Context, db redb.RedisDB, args *VerifyReputationArgs) (ranking, []Extra, error) {
	target, err := db.NodeByKey(ctx, args.Target)
	if err != nil {
		if errors.Is(err, graph.ErrNodeNotFound) {
			// target is not found, assume it's a low-reputation key (rank of 0)
			return ranking{{Key: args.Target, Val: 0}}, nil, nil
		}
		return nil, nil, err
	}

	followers, err := db.Followers(ctx, target.ID)
	if err != nil {
		return nil, nil, err
	}

	toRank := append([]graph.ID{target.ID}, followers...)
	nodeRanking, err := rankNodes(ctx, db, toRank, args.Algorithm)
	if err != nil {
		return nil, nil, err
	}

	// place target in the first position, regardless of its rank
	nodeRanking = append(nodeRanking[0:1], nodeRanking[1:].MaxK(args.Limit)...)
	ranking, err := resolveIDs(ctx, db, nodeRanking)
	if err != nil {
		return nil, nil, err
	}

	follows, err := db.FollowCounts(ctx, target.ID)
	if err != nil {
		return nil, nil, err
	}

	followCount := follows[0]
	followerCount := len(followers)

	extras := []Extra{{
		Follows:   &followCount,
		Followers: &followerCount,
	}}

	return ranking, extras, nil
}

// RankProfiles returns the rank of each specified target.
// All ranks use the specified args.Algorithm.
// For more info read: https://vertexlab.io/docs/services/sort-profiles/
func RankProfiles(ctx context.Context, db redb.RedisDB, request *Request) (Response, error) {
	args, err := request.ToRankProfilesArgs()
	if err != nil {
		return nil, err
	}

	ranking, err := rankProfiles(ctx, db, args)
	if err != nil {
		return nil, fmt.Errorf("RankProfiles %w: %w", ErrInternal, err)
	}
	return NewResponse(ranking), nil
}

func rankProfiles(ctx context.Context, db redb.RedisDB, args *RankProfilesArgs) (ranking, error) {
	targets, err := db.NodeIDs(ctx, args.Targets...)
	if err != nil {
		return nil, err
	}

	ranks, err := rank(ctx, db, targets, args.Algorithm)
	if err != nil {
		return nil, err
	}

	ranking := slicex.Pack(args.Targets, ranks)
	return ranking.MaxK(args.Limit), nil
}

// SearchProfiles returns the top ranked pubkeys whose kind:0s contain the provided string.
// All ranks use the specified args.Algorithm.
// For more info read: https://vertexlab.io/docs/services/search-profiles/
func SearchProfiles(ctx context.Context, db redb.RedisDB, store *sqlite.Store, request *Request) (Response, error) {
	args, err := request.ToSearchProfilesArgs()
	if err != nil {
		return nil, err
	}

	ranking, err := searchProfiles(ctx, db, store, args)
	if err != nil {
		return nil, fmt.Errorf("SearchProfiles %w: %w", ErrInternal, err)
	}
	return NewResponse(ranking), nil
}

func searchProfiles(ctx context.Context, db redb.RedisDB, store *sqlite.Store, args *SearchProfilesArgs) (ranking, error) {
	ranking, err := fts5(ctx, store, args.Search)
	if err != nil {
		return nil, err
	}

	pubkeys, searchRanks := ranking.Unpack()
	nodes, err := db.NodeIDs(ctx, pubkeys...)
	if err != nil {
		return nil, err
	}

	reputations, err := rank(ctx, db, nodes, args.Algorithm)
	if err != nil {
		return nil, err
	}

	for i := range reputations {
		// merge reputational and search ranks
		ranking[i].Val = math.Pow(searchRanks[i], 3) * reputations[i]
	}
	return ranking.MaxK(args.Limit), nil
}

// fts5 performs full text seach on the profiles (kind:0s) using the specified search term.
// It returns the pubkeys and search scores (positives, higher is better) of the SQL query.
func fts5(ctx context.Context, store *sqlite.Store, search string) (ranking, error) {
	if pk, err := ToHexPubkey(search); err == nil {
		// the search term is a pubkey or npub, so we don't search further
		return slicex.Pairs[string, float64]{{Key: pk, Val: 1}}, nil
	}

	search = escapeFTS5(search)
	var matches int

	row := store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM profiles_fts WHERE profiles_fts MATCH ?", search)
	if err := row.Scan(&matches); err != nil {
		return nil, fmt.Errorf("failed to count matches: %w", err)
	}

	var d, limit = dampening(matches), min(matches, maxSearchLimit)
	var name, displayName, about, website, nip05 float64 = 10, 12, 1 * d, 1 * d, 3 * d

	query := `SELECT pubkey, bm25(profiles_fts, 0.0, 0.0, ?, ?, ?, ?, ?) AS score
				FROM profiles_fts
				WHERE profiles_fts MATCH ? AND score < 0
				ORDER BY score
				LIMIT ?;`

	rows, err := store.DB.QueryContext(ctx, query, name, displayName, about, website, nip05, search, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to make full text search: %w", err)
	}
	defer rows.Close()

	ranking := make(ranking, 0, limit) // pre-allocating
	var pk string
	var rank float64

	for rows.Next() {
		if err = rows.Scan(&pk, &rank); err != nil {
			return nil, fmt.Errorf("failed to scan the results of the query: %w", err)
		}

		// convert bm25 scores (negative) to positive to have best is highest
		ranking = append(ranking, slicex.Pair[string, float64]{Key: pk, Val: -rank})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan the results of the query: %w", err)
	}
	return ranking, nil
}

// escapeFTS5 prepares a search term for SQLite FTS5
func escapeFTS5(term string) string {
	term = strings.ReplaceAll(term, `"`, `""`)
	return `"` + term + `"`
}

// The default and max `LIMIT` for the full-text-search query.
const (
	defaultSearchLimit = 700
	maxSearchLimit     = 3000
)

/*
This function returns the dampening coefficient used to decrease the importance of the
'about', 'website', 'nip05' columns when performing full-text search.

The rationale is the following: the higher the 'matches', the lower the weight of such columns.
When matches surpasses [defaultSearchLimit] (the budget of the query), the coefficient goes to 0.
This behaviour is useful for searches involving popular nip05/lightning providers (e.g. 'primal', 'alby'),
or common terms like 'bitcoin' and 'nostr'.
*/
func dampening(matches int) float64 {
	m, l := float64(matches), float64(defaultSearchLimit)
	return math.Max(1-math.Pow(m/l, 2), 0)
}

// RecommendFollows uses the specified [Algorithm] to return a list of recommendations for args.Source.
// The recommended pubkeys are the highest ranked, excluding args.Source and its follows (if any).
// For more info read: https://vertexlab.io/docs/services/recommend-follows/
func RecommendFollows(ctx context.Context, db redb.RedisDB, request *Request) (Response, error) {
	args, err := request.ToRecommendFollowsArgs()
	if err != nil {
		return nil, err
	}

	ranking, err := recommendFollows(ctx, db, args)
	if err != nil {
		return nil, fmt.Errorf("RecommendFollows %w: %w", ErrInternal, err)
	}
	return NewResponse(ranking), nil
}

func recommendFollows(ctx context.Context, db redb.RedisDB, args *RecommendFollowsArgs) (ranking, error) {
	var nodeRanking nodeRanking
	var err error

	switch args.Sort {
	case Global:
		nodeRanking, err = recommendByGlobal(ctx, db, args)

	case Personalized:
		nodeRanking, err = recommendByPersonalized(ctx, db, args)

	case Followers:
		nodeRanking, err = recommendByFollowers(ctx, db, args)

	default:
		err = ErrInvalidSort
	}

	if err != nil {
		return nil, fmt.Errorf("failed to recommend with %s: %w", args.Sort, err)
	}
	return resolveIDs(ctx, db, nodeRanking)
}

func recommendByGlobal(ctx context.Context, db redb.RedisDB, args *RecommendFollowsArgs) (nodeRanking, error) {
	var avoid []graph.ID
	node, err := db.NodeByKey(ctx, args.Source)
	switch {
	case errors.Is(err, graph.ErrNodeNotFound):
		// we can still recommend, continue

	case err != nil:
		// issue with the database, fail
		return nil, err

	default:
		// remove follows and self from the recommendations
		follows, err := db.Follows(ctx, node.ID)
		if err != nil {
			return nil, err
		}

		avoid = append(follows, node.ID)
	}

	// assumption: the first 50k nodes are the highest ranked. TODO: improve
	candidates := make([]graph.ID, 50000)
	for i := range 50000 {
		candidates[i] = graph.ID(strconv.FormatInt(int64(i), 10))
	}

	candidates = slicex.Difference(candidates, avoid)
	ranks, err := pagerank.Global(ctx, db, candidates...)
	if err != nil {
		return nil, err
	}

	nodeRanking := slicex.Pack(candidates, ranks)
	return nodeRanking.MaxK(args.Limit), nil
}

func recommendByFollowers(ctx context.Context, db redb.RedisDB, args *RecommendFollowsArgs) (nodeRanking, error) {
	var avoid []graph.ID
	node, err := db.NodeByKey(ctx, args.Source)
	switch {
	case errors.Is(err, graph.ErrNodeNotFound):
		// we can still recommend, continue

	case err != nil:
		// issue with the database, fail
		return nil, err

	default:
		// remove follows and self from the recommendations
		follows, err := db.Follows(ctx, node.ID)
		if err != nil {
			return nil, err
		}

		avoid = append(follows, node.ID)
	}

	// assumption: the first 50k nodes are the highest ranked. TODO: improve
	candidates := make([]graph.ID, 50000)
	for i := range 50000 {
		candidates[i] = graph.ID(strconv.FormatInt(int64(i), 10))
	}

	candidates = slicex.Difference(candidates, avoid)
	counts, err := db.FollowerCounts(ctx, candidates...)
	if err != nil {
		return nil, err
	}

	nodeRanking := make(nodeRanking, len(candidates))
	for i, node := range candidates {
		nodeRanking[i] = slicex.Pair[graph.ID, float64]{Key: node, Val: float64(counts[i])}
	}
	return nodeRanking.MaxK(args.Limit), nil
}

func recommendByPersonalized(ctx context.Context, db redb.RedisDB, args *RecommendFollowsArgs) (nodeRanking, error) {
	source, err := db.NodeByKey(ctx, args.Source)
	if err != nil {
		return nil, err
	}

	follows, err := db.Follows(ctx, source.ID)
	if err != nil {
		return nil, err
	}

	pp, err := pagerank.Personalized(ctx, db, source.ID, 100_000)
	if err != nil {
		return nil, err
	}

	// remove follows and self from the recommendations
	for _, ID := range append(follows, source.ID) {
		delete(pp, ID)
	}

	nodeRanking := slicex.ToPairs(pp)
	return nodeRanking.MaxK(args.Limit), nil
}

func rankNodes(ctx context.Context, db redb.RedisDB, nodes []graph.ID, algo Algorithm) (nodeRanking, error) {
	ranks, err := rank(ctx, db, nodes, algo)
	if err != nil {
		return nil, err
	}
	return slicex.Pack(nodes, ranks), nil
}

// rank the nodes according to the provided [Algorithm].
// If a node is not found, the rank is always assumed to be 0.
func rank(ctx context.Context, db redb.RedisDB, nodes []graph.ID, algo Algorithm) ([]float64, error) {
	switch algo.Sort {
	case Followers:
		counts, err := db.FollowerCounts(ctx, nodes...)
		if err != nil {
			return nil, err
		}

		ranks := make([]float64, len(counts))
		for i, count := range counts {
			ranks[i] = float64(count)
		}
		return ranks, nil

	case Global:
		return pagerank.Global(ctx, db, nodes...)

	case Personalized:
		source, err := db.NodeByKey(ctx, algo.Source)
		if err != nil {
			return nil, err
		}

		return pagerank.PersonalizedWithTargets(ctx, db, source.ID, nodes, 100_000)

	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidSort, algo.Sort)
	}
}

// resolveIDs is used for converting a [nodeRanking] into a [ranking].
// If an ID is not found, it returns an error.
func resolveIDs(
	ctx context.Context,
	db redb.RedisDB,
	nodeRanking nodeRanking) (ranking, error) {

	IDs, ranks := nodeRanking.Unpack()
	pubkeys, err := db.Pubkeys(ctx, IDs...)
	if err != nil {
		return nil, err
	}

	ranking := make(ranking, len(IDs))
	for i, pk := range pubkeys {
		if pk == "" {
			return nil, fmt.Errorf("failed to fetch the pubkey of node ID %s", IDs[i])
		}

		ranking[i] = slicex.Pair[string, float64]{Key: pk, Val: ranks[i]}
	}

	return ranking, nil
}
