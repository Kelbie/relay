// The dvm package is responsible for parsing and responding to DVM requests.
package dvm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/crawler/pkg/models"
	"github.com/vertex-lab/crawler/pkg/pagerank"
	"github.com/vertex-lab/crawler/pkg/utils/sliceutils"
	"github.com/vertex-lab/relay/pkg/eventstore"
	"github.com/vertex-lab/relay/pkg/pairs"
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
	Ranking     = pairs.Pairs[string, float64] // a slice of (pubkey, rank)
	nodeRanking = pairs.Pairs[uint32, float64] // a slice of (nodeID, rank)
)

// NewResponse() combines the ranking and the optional extras into a [Response].
func NewResponse(ranking Ranking, extras ...Extra) Response {
	res := make(Response, len(ranking))
	for i, pair := range ranking {
		res[i] = ResponseItem{Pubkey: pair.Key, Rank: pair.Val}
	}

	for i, extra := range extras {
		res[i].Extra = extra
	}

	return res
}

// Pubkeys() returns the pubkeys present in the response.
func (r Response) Pubkeys() []string {
	pubkeys := make([]string, len(r))
	for i, item := range r {
		pubkeys[i] = item.Pubkey
	}

	return pubkeys
}

// ErrorEvent() returns an unsigned nostr event for the DVM error
func ErrorEvent(err error, rec Record) *nostr.Event {
	return &nostr.Event{
		CreatedAt: nostr.Now(),
		Kind:      KindDVMError,
		Tags:      append(rec.ToTags(), nostr.Tag{"status", "error", err.Error()}),
	}
}

// ResponseEvent() returns an unsigned nostr event used for the DVM.
// The `CreatedAt` field in the response event shows how old the ranking data is.
func ResponseEvent(res Response, req *Request) *nostr.Event {
	if len(res) >= 1 && req.Kind == KindVerifyReputation && req.ID == "" {
		// this is a nasty trick to mantain backwards compatibility with Zapstore,
		// that should be removed as soon as Zapstore upgrades to the new format for VerifyReputation.
		// rec.ID == "" iff REQ is used.
		res = res[1:]
	}

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

// VerifyReputation() returns the rank of the target and its highest ranked followers.
// All ranks use the specified [Algorithm].
// For more info read: https://vertexlab.io/docs/services/verify-reputation/
func VerifyReputation(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	request *Request) (Response, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	args, err := request.ToVerifyReputationArgs()
	if err != nil {
		return nil, err
	}

	ranking, extras, err := verifyReputation(ctx, DB, RWS, args)
	if err != nil {
		return nil, fmt.Errorf("VerifyReputation %w: %w", ErrInternal, err)
	}

	return NewResponse(ranking, extras...), nil
}

func verifyReputation(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *VerifyReputationArgs) (Ranking, []Extra, error) {

	target, err := DB.NodeByKey(ctx, args.Target)
	if errors.Is(err, models.ErrNodeNotFoundDB) {
		// if target is not found in our database, we assume it's a low-reputation key (rank of 0).
		return Ranking{{Key: args.Target, Val: 0}}, nil, nil
	}

	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch the target's ID: %w", err)
	}

	IDs, err := DB.Followers(ctx, target.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch the followers of the target: %w", err)
	}

	followerIDs := IDs[0]
	nodesToRank := append([]uint32{target.ID}, followerIDs...)

	nodeRanking, err := rankNodes(ctx, DB, RWS, nodesToRank, args.Algorithm)
	if err != nil {
		return nil, nil, err
	}

	topFollowers := nodeRanking[1:].Top(args.Limit)
	nodeRanking = append(nodeRanking[0:1], topFollowers...) // the response MUST have the target in the first position

	ranking, err := resolveIDs(ctx, DB, nodeRanking)
	if err != nil {
		return nil, nil, err
	}

	follows, err := DB.FollowCounts(ctx, target.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch the follows of the target: %w", err)
	}

	followCount := follows[0]
	followerCount := len(followerIDs)

	extras := []Extra{{
		Follows:   &followCount,
		Followers: &followerCount,
	}}

	return ranking, extras, nil
}

// RankProfiles() returns the rank of each specified target.
// All ranks use the specified args.Algorithm.
// For more info read: https://vertexlab.io/docs/services/sort-profiles/
func RankProfiles(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	request *Request) (Response, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	args, err := request.ToRankProfilesArgs()
	if err != nil {
		return nil, err
	}

	ranking, err := rankProfiles(ctx, DB, RWS, args)
	if err != nil {
		return nil, fmt.Errorf("RankProfiles %w: %w", ErrInternal, err)
	}

	return NewResponse(ranking), nil
}

func rankProfiles(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *RankProfilesArgs) (Ranking, error) {

	IDs, err := DB.NodeIDs(ctx, args.Targets...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the IDs of the targets: %w", err)
	}

	var targetIDs = make([]uint32, len(IDs))
	for i, ID := range IDs {
		if ID == nil {
			// if target is not found in our database, we assume it's a low-reputation key (rank of 0).
			// To do so while maintaining syncronization with the args.Targets, we assign it the signalling value MaxUint32.
			targetIDs[i] = math.MaxUint32
		} else {
			targetIDs[i] = *ID
		}
	}

	targetRanking, err := rankNodes(ctx, DB, RWS, targetIDs, args.Algorithm)
	if err != nil {
		return nil, err
	}

	_, ranks := targetRanking.Unpack()
	ranking, err := pairs.Pack(args.Targets, ranks)
	if err != nil {
		return nil, err
	}

	return ranking.Top(args.Limit), nil
}

// SearchProfiles() returns the top ranked pubkeys whose kind:0s contain the provided string.
// All ranks use the specified args.Algorithm.
// For more info read: https://vertexlab.io/docs/services/search-profiles/
func SearchProfiles(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	eventStore *eventstore.Store,
	request *Request) (Response, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	args, err := request.ToSearchProfilesArgs()
	if err != nil {
		return nil, err
	}

	ranking, err := searchProfiles(ctx, DB, RWS, eventStore, args)
	if err != nil {
		return nil, fmt.Errorf("SearchProfiles %w: %w", ErrInternal, err)
	}

	return NewResponse(ranking), nil
}

func searchProfiles(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	eventStore *eventstore.Store,
	args *SearchProfilesArgs) (Ranking, error) {

	ranking, err := fts5(ctx, eventStore, args.Search)
	if err != nil {
		return nil, err
	}

	pubkeys, searchRanks := ranking.Unpack()
	IDs, err := DB.NodeIDs(ctx, pubkeys...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the IDs of search results: %w", err)
	}

	targetIDs := make([]uint32, len(IDs))
	for i, ID := range IDs {
		if ID == nil {
			// if target is not found in our database, we assume it's a low-reputation key (rank of 0).
			// To do so while maintaining syncronization with the args.Targets, we assign it the signalling value MaxUint32.
			// TODO; We should log when this happens.
			targetIDs[i] = math.MaxUint32
		} else {
			targetIDs[i] = *ID
		}
	}

	targetRanking, err := rankNodes(ctx, DB, RWS, targetIDs, args.Algorithm)
	if err != nil {
		return nil, err
	}

	for i, target := range targetRanking {
		// merge ranks and searchRanks in order to give more accurate search results
		ranking[i].Val = math.Pow(searchRanks[i], 3) * target.Val
	}

	return ranking.Top(args.Limit), nil
}

// The function fts5() performs full text seach on the profiles (kind:0s) using the specified search term.
// It returns the pubkeys and search scores (positives, higher is better) of the SQL query.
func fts5(
	ctx context.Context,
	eventStore *eventstore.Store,
	search string) (Ranking, error) {

	if pk, err := ToHexPubkey(search); err == nil {
		// if the search term IS a pubkey, then we don't search further
		return pairs.Pairs[string, float64]{{Key: pk, Val: 1}}, nil
	}

	search = escapeFTS5(search)
	var matches int

	row := eventStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM profiles_fts WHERE profiles_fts MATCH ?", search)
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

	rows, err := eventStore.DB.QueryContext(ctx, query, name, displayName, about, website, nip05, search, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query the database: %w", err)
	}
	defer rows.Close()

	ranking := make(Ranking, 0, limit) // pre-allocating
	var pk string
	var rank float64

	for rows.Next() {
		if err = rows.Scan(&pk, &rank); err != nil {
			return nil, fmt.Errorf("failed to scan the results of the query: %w", err)
		}

		// bm25 scores are all negative but we prefer to have positive scores, since we need the top from highest to lowest
		ranking = append(ranking, pairs.Pair[string, float64]{Key: pk, Val: -rank})
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
When matches surpasses `defaultSearchLimit` (the budget for the query), the coefficient goes to 0.
This behaviour is useful for searches involving popular nip05/lightning providers (e.g. 'primal', 'alby'),
or common terms like 'bitcoin' and 'nostr'.
*/
func dampening(matches int) float64 {
	m, l := float64(matches), float64(defaultSearchLimit)
	return math.Max(1-math.Pow(m/l, 2), 0)
}

// RecommendFollows() uses the specified args.Algorithm to return a list of recommendations for args.Source.
// The recommended pubkeys are the highest ranked, excluding args.Source and its follows (if any).
// For more info read: https://vertexlab.io/docs/services/recommend-follows/
func RecommendFollows(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	request *Request) (Response, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	args, err := request.ToRecommendFollowsArgs()
	if err != nil {
		return nil, err
	}

	ranking, err := recommendFollows(ctx, DB, RWS, args)
	if err != nil {
		return nil, fmt.Errorf("RecommendFollows %w: %w", ErrInternal, err)
	}

	return NewResponse(ranking), nil
}

func recommendFollows(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *RecommendFollowsArgs) (Ranking, error) {

	var nodeRanking nodeRanking
	var err error

	switch args.Sort {
	case Global:
		nodeRanking, err = recommendByGlobal(ctx, DB, RWS, args.Source, args.Limit)

	case Personalized:
		nodeRanking, err = recommendByPersonalized(ctx, DB, RWS, args.Source, args.Limit)

	case Followers:
		nodeRanking, err = recommendByFollowers(ctx, DB, args.Source, args.Limit)

	default:
		err = fmt.Errorf("%w: %s", ErrInvalidSort, args.Sort)
	}

	if err != nil {
		return nil, err
	}

	return resolveIDs(ctx, DB, nodeRanking)
}

func recommendByGlobal(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	source string,
	limit int) (nodeRanking, error) {

	var avoid []uint32 // a slice of nodeIDs that should not be recommended, like self, follows, mutes...
	node, err := DB.NodeByKey(ctx, source)

	switch {
	case errors.Is(err, models.ErrNodeNotFoundDB):
		// do nothing, as we can still recommend.

	case err != nil:
		// this means there are issue with our DB, so it's better to fail.
		return nil, fmt.Errorf("failed to fetch the ID of source: %w", err)

	default:
		follows, err := DB.Follows(ctx, node.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch follows of %s", source)
		}

		avoid = append(follows[0], node.ID) // remove follows and self from the recommendations
	}

	// this should be faster than using DB.AllNodes(). It might happen that some nodeIDs
	// are not associated with any node, but this is not a problem since their pagerank will be 0.
	candidates := make([]uint32, DB.Size(ctx))
	for i := 0; i < len(candidates); i++ {
		candidates[i] = uint32(i)
	}

	candidates = sliceutils.Difference(candidates, avoid)
	ranks, err := pagerank.Global(ctx, RWS, candidates...)
	if err != nil {
		return nil, fmt.Errorf("failed to recommend with %s: %w", Global, err)
	}

	return pairs.Top(ranks, limit), nil
}

func recommendByPersonalized(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	source string,
	limit int) (nodeRanking, error) {

	node, err := DB.NodeByKey(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the ID of source: %w", err)
	}

	follows, err := DB.Follows(ctx, node.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch follows of %s", source)
	}

	avoid := append(follows[0], node.ID) // remove follows and self from the recommendations
	pp, err := pagerank.Personalized(ctx, DB, RWS, node.ID, 50)
	if err != nil {
		return nil, err
	}

	for _, ID := range avoid {
		delete(pp, ID)
	}

	return pairs.Top(pp, limit), nil
}

func recommendByFollowers(
	ctx context.Context,
	DB models.Database,
	source string,
	limit int) (nodeRanking, error) {

	var avoid []uint32 // a slice of nodeIDs that should not be recommended, like self, follows, mutes...
	node, err := DB.NodeByKey(ctx, source)

	switch {
	case errors.Is(err, models.ErrNodeNotFoundDB):
		// do nothing, as we can still recommend.

	case err != nil:
		// this means there are issue with our DB, so it's better to fail.
		return nil, fmt.Errorf("failed to fetch the ID of source: %w", err)

	default:
		follows, err := DB.Follows(ctx, node.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch follows of %s", source)
		}

		avoid = append(follows[0], node.ID) // remove follows and self from the recommendations
	}

	// this should be faster than using DB.AllNodes(). It might happen that some nodeIDs
	// are not associated with any node, but this is not a problem since their followers will be 0.
	candidates := make([]uint32, DB.Size(ctx))
	for i := 0; i < len(candidates); i++ {
		candidates[i] = uint32(i)
	}

	candidates = sliceutils.Difference(candidates, avoid)
	ranks, err := DB.FollowerCounts(ctx, candidates...)
	if err != nil {
		return nil, fmt.Errorf("failed to recommend with %s: %w", Followers, err)
	}

	nodeRanking := make(nodeRanking, len(candidates))
	for i, ID := range candidates {
		nodeRanking[i] = pairs.Pair[uint32, float64]{Key: ID, Val: float64(ranks[i])}
	}

	return nodeRanking.Top(limit), nil
}

// rankNodes() associates a rank to each node ID by applying the specified algorithm.
// If the nodeID is not found, the rank is always assumed to be 0.
func rankNodes(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	IDs []uint32,
	algo Algorithm) (nodeRanking, error) {

	if len(IDs) == 0 {
		return nil, nil
	}

	switch algo.Sort {
	case Global:
		return rankByGlobal(ctx, RWS, IDs)

	case Personalized:
		return rankByPersonalized(ctx, DB, RWS, IDs, algo.Source)

	case Followers:
		return rankByFollowers(ctx, DB, IDs)

	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidSort, algo.Sort)
	}
}

func rankByGlobal(ctx context.Context, RWS models.RandomWalkStore, IDs []uint32) (nodeRanking, error) {
	ranks, err := pagerank.Global(ctx, RWS, IDs...)
	if err != nil {
		return nil, fmt.Errorf("failed to sort with '%s': %w", Global, err)
	}

	nodeRanking := make(nodeRanking, len(IDs))
	for i, ID := range IDs {
		nodeRanking[i] = pairs.Pair[uint32, float64]{Key: ID, Val: ranks[ID]}
	}

	return nodeRanking, nil
}

func rankByPersonalized(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	IDs []uint32,
	source string) (nodeRanking, error) {

	sourceID, err := DB.NodeIDs(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("failed to sort with '%s': %w", Personalized, err)
	}

	if sourceID[0] == nil {
		return nil, fmt.Errorf("%w: pubkey was not found", ErrInvalidSource)
	}

	ranks, err := pagerank.Personalized(ctx, DB, RWS, *sourceID[0], 100)
	if err != nil {
		return nil, fmt.Errorf("failed to sort with '%s': %w", Personalized, err)
	}

	nodeRanking := make(nodeRanking, len(IDs))
	for i, ID := range IDs {
		nodeRanking[i] = pairs.Pair[uint32, float64]{Key: ID, Val: ranks[ID]}
	}

	return nodeRanking, nil
}

func rankByFollowers(ctx context.Context, DB models.Database, IDs []uint32) (nodeRanking, error) {
	ranks, err := DB.FollowerCounts(ctx, IDs...)
	if err != nil {
		return nil, fmt.Errorf("failed to sort with '%s': %w", Followers, err)
	}

	nodeRanking := make(nodeRanking, len(IDs))
	for i, ID := range IDs {
		nodeRanking[i] = pairs.Pair[uint32, float64]{Key: ID, Val: float64(ranks[i])}
	}

	return nodeRanking, nil
}

// -----------------------------------HELPERS-----------------------------------

// resolveIDs() is used for converting the nodeIDs of Pairs[uint32, float64] into pubkeys.
// In other words, it converts [nodeRanking] into a [Ranking].
func resolveIDs(
	ctx context.Context,
	DB models.Database,
	nodeRanks nodeRanking) (Ranking, error) {

	IDs, ranks := nodeRanks.Unpack()
	pubkeys, err := DB.Pubkeys(ctx, IDs...)
	if err != nil {
		return nil, err
	}

	ranking := make(Ranking, len(IDs))
	for i, pk := range pubkeys {
		if pk == nil {
			return nil, fmt.Errorf("failed to fetch the pubkey of nodeID %d", IDs[i])
		}

		ranking[i] = pairs.Pair[string, float64]{Key: *pk, Val: ranks[i]}
	}

	return ranking, nil
}
