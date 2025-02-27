package dvm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/vertex-lab/crawler/pkg/models"
	"github.com/vertex-lab/crawler/pkg/pagerank"
	"github.com/vertex-lab/relay/pkg/eventstore"
)

// RankResponse (for type) returns the rank for the requested pubkey.
type RankResponse struct {
	Pubkey string  `json:"pubkey"`
	Rank   float64 `json:"rank"`
}

type RankResponses []RankResponse

func (r RankResponses) Unpack() ([]string, []float64) {
	pubkeys := make([]string, len(r))
	ranks := make([]float64, len(r))

	for i, res := range r {
		pubkeys[i] = res.Pubkey
		ranks[i] = res.Rank
	}

	return pubkeys, ranks
}

// VerifyReputation() returns the rank of the target and its highest ranked followers.
// All ranks use the specified args.Sort algorithm.
// For more info read: https://vertexlab.io/docs/nips/verify-reputation-dvm/
func VerifyReputation(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *Args) (RankResponses, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := validateVerifyReputation(args); err != nil {
		return nil, fmt.Errorf("VerifyReputation: %w", err)
	}

	IDs, err := DB.NodeIDs(ctx, args.Targets[0], args.Source)
	if err != nil {
		return nil, fmt.Errorf("VerifyReputation: failed to fetch the IDs of source and target: %w", err)
	}

	target, source := IDs[0], IDs[1]
	if target == nil {
		// if target is not found in our database, we assume it's a low-reputation key. This heuristic is based
		// on the fact that to be added to our DB, a key requires only one follows from an active node.
		return RankResponses{{Pubkey: args.Targets[0], Rank: 0}}, nil
	}

	followers, err := DB.Followers(ctx, *target)
	if err != nil {
		return nil, fmt.Errorf("VerifyReputation: failed to fetch the followers of target: %w", err)
	}

	toRank := append(followers[0], *target)
	rankMap, err := rankNodes(ctx, DB, RWS, toRank, args.Sort, source)
	if err != nil {
		return nil, fmt.Errorf("VerifyReputation: %w", err)
	}

	// Target must be first in the response, then its highest ranked followers.
	// To avoid the possibility of target being in the second group as well, we remove its key from the rankMap.
	nodeRanks := pairs{{ID: *target, rank: rankMap[*target]}}
	delete(rankMap, *target)
	nodeRanks = append(nodeRanks, topPairs(rankMap, args.Limit)...)

	res, err := buildResponse(ctx, DB, nodeRanks)
	if err != nil {
		return nil, fmt.Errorf("VerifyReputation: %w", err)
	}

	return res, nil
}

// SortAuthors() returns the rank of each specified target.
// All ranks use the specified args.Sort algorithm.
// For more info read: https://vertexlab.io/docs/nips/sort-authors-dvm/
func SortAuthors(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *Args) (RankResponses, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := validateSortAuthors(args); err != nil {
		return nil, fmt.Errorf("SortAuthors: %w", err)
	}

	args.Limit = min(args.Limit, len(args.Targets))

	IDs, err := DB.NodeIDs(ctx, append(args.Targets, args.Source)...)
	if err != nil {
		return nil, fmt.Errorf("SortAuthors: failed to fetch the IDs of source and targets: %w", err)
	}

	var sourceID *uint32 = IDs[len(IDs)-1]
	var targetIDs []uint32
	var notFound []string
	for i := 0; i < len(IDs)-1; i++ {
		if IDs[i] != nil {
			targetIDs = append(targetIDs, *IDs[i])
		} else {
			notFound = append(notFound, args.Targets[i])
		}
	}

	ranks, err := rankNodes(ctx, DB, RWS, targetIDs, args.Sort, sourceID)
	if err != nil {
		return nil, fmt.Errorf("SortAuthors: %w", err)
	}

	top := topPairs(ranks, args.Limit)
	res, err := buildResponse(ctx, DB, top)
	if err != nil {
		return nil, fmt.Errorf("SortAuthors: %w", err)
	}

	var i int
	for len(res) < args.Limit {
		// if the response is shorter than `limit`, "pad" it with pubkeys that are not found
		// as always, the assumption is that, if a key was not found in our DB, it's not reputable
		res = append(res, RankResponse{Pubkey: notFound[i], Rank: 0.0})
		i++
	}

	return res, nil
}

// SearchAuthors() returns the top ranked pubkeys whose kind:0s contain the provided string.
// All ranks use the specified args.Sort algorithm.
// For more info read:
func SearchAuthors(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	eventStore *eventstore.Store,
	args *Args) (RankResponses, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := validateSearchAuthors(args); err != nil {
		return nil, fmt.Errorf("SearchAuthors: %w", err)
	}

	pubkeys, searchRanks, err := searchAuthors(ctx, eventStore, args.Search)
	if err != nil {
		return nil, fmt.Errorf("SearchAuthors: %w", err)
	}

	if len(pubkeys) == 0 {
		// if the search did not find anything, return an empty response
		return RankResponses{}, nil
	}

	IDs, err := DB.NodeIDs(ctx, append(pubkeys, args.Source)...)
	if err != nil {
		return nil, fmt.Errorf("SearchAuthors: failed to fetch the IDs of search results: %w", err)
	}

	var sourceID *uint32 = IDs[len(IDs)-1]
	targetIDs := make([]uint32, 0, len(IDs)-1)
	for i := 0; i < len(IDs)-1; i++ {
		if IDs[i] == nil {
			// Add signalling value MaxUint32 so that rankNodes returns 0, while keeping sincronisation with the pubkeys slice.
			// TODO; We should log if the search returns pubkeys that aren't found in redis.
			targetIDs = append(targetIDs, math.MaxUint32)
		} else {
			targetIDs = append(targetIDs, *IDs[i])
		}
	}

	ranks, err := rankNodes(ctx, DB, RWS, targetIDs, args.Sort, sourceID)
	if err != nil {
		return nil, fmt.Errorf("SearchAuthors: %w", err)
	}

	for i, ID := range targetIDs {
		// merge ranks and searchRanks in order to give more accurate search results
		ranks[ID] = ranks[ID] * math.Pow(searchRanks[i], 3)
	}

	top := topPairs(ranks, args.Limit)
	res, err := buildResponse(ctx, DB, top)
	if err != nil {
		return nil, fmt.Errorf("SortAuthors: %w", err)
	}

	return res, nil
}

// RecommendFollows() uses the specified args.Sort algorithm to return a list of recommendations for args.Source.
// The recommended pubkeys are the highest ranked, excluding args.Source and its follows (if any).
// For more info read: https://vertexlab.io/docs/nips/recommend-follows-dvm/
func RecommendFollows(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *Args) (RankResponses, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := validateRecommendFollows(args); err != nil {
		return nil, fmt.Errorf("RecommendFollows: %w", err)
	}

	var ranks models.PagerankMap
	var err error
	switch args.Sort {
	case "globalPagerank":
		ranks, err = recommendFollowsGlobal(ctx, DB, RWS, args.Source)

	case "personalizedPagerank":
		limit := int(min(args.Limit, 30))
		ranks, err = recommendFollowsPersonalized(ctx, DB, RWS, args.Source, limit)

	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidSortOption, args.Sort)
	}

	if err != nil {
		return nil, fmt.Errorf("RecommendFollows: %w", err)
	}

	top := topPairs(ranks, int(args.Limit))
	res, err := buildResponse(ctx, DB, top)
	if err != nil {
		return nil, fmt.Errorf("RecommendFollows: %w", err)
	}

	return res, nil
}

// The function searchAuthors() performs full text seach on the profiles (kind:0s) using the specified search term.
// It returns the pubkeys and search scores (positives, higher is better) of the SQL query.
func searchAuthors(ctx context.Context, eventStore *eventstore.Store, search string) (pubkeys []string, scores []float64, err error) {

	search = strings.TrimSpace(search)
	if len(search) < 3 {
		// since we are using the trigram 'tokenizer', we know there won't be any matches.
		return nil, nil, nil
	}

	var matches int
	err = eventStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM profiles_fts WHERE profiles_fts MATCH ?", search).Scan(&matches)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to count matches: %w", err)
	}

	var d, limit = dampening(matches), limit(matches)
	var name, displayName, about, website, nip05 float64 = 10, 12, 1 * d, 1 * d, 3 * d

	query := `
	SELECT 
		pubkey, 
		bm25(profiles_fts, 0.0, 0.0, ?, ?, ?, ?, ?) AS score
	FROM profiles_fts
	WHERE profiles_fts MATCH ?
	ORDER BY score
	LIMIT ?;`

	rows, err := eventStore.DB.QueryContext(ctx, query, name, displayName, about, website, nip05, search, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query the database: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var pubkey string
		var score float64
		if err = rows.Scan(&pubkey, &score); err != nil {
			return nil, nil, fmt.Errorf("failed to scan the results of the query: %w", err)
		}

		pubkeys = append(pubkeys, pubkey)
		scores = append(scores, -score) // bm25 scores are all negative but we prefer to have positive scores
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("failed to scan the results of the query: %w", err)
	}

	return pubkeys, scores, nil
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
This behaviour is useful for searches involving popular nip05 providers (e.g. 'primal'),
or common terms like 'bitcoin' and 'nostr'.
*/
func dampening(matches int) float64 {
	m, l := float64(matches), float64(defaultSearchLimit)
	return math.Max(1-math.Pow(m/l, 2), 0)
}

// This function returns the `limit` to be used for the full-text search query.
// It is elastic in the number of `matches`, but no smaller than `defaultSearchLimit` and
// no bigger than `maxSearchLimit`.
func limit(matches int) int {
	return max(defaultSearchLimit, min(matches/4, maxSearchLimit))
}

// rankNodes() associates a rank to each target by applying the specified algorithm.
// If the algorithm is personalizedPagerank, it uses the provided source.
func rankNodes(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	targets []uint32,
	algorithm string,
	source *uint32) (models.PagerankMap, error) {

	if len(targets) == 0 {
		return nil, fmt.Errorf("%w: empty targets", ErrInvalidTargets)
	}

	var ranks models.PagerankMap
	var err error

	switch algorithm {
	case "globalPagerank":
		ranks, err = pagerank.Global(ctx, RWS, targets...)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to sort with globalPagerank pagerank %v", ErrComputationFailed, err)
		}

	case "personalizedPagerank":
		if source == nil {
			return nil, fmt.Errorf("source %w", ErrKeyNotFound)
		}

		pp, err := pagerank.Personalized(ctx, DB, RWS, *source, 100)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to sort with personalizedPagerank pagerank %v", ErrComputationFailed, err)
		}

		ranks = make(models.PagerankMap, len(targets))
		for _, ID := range targets {
			ranks[ID] = pp[ID]
		}

	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidSortOption, algorithm)
	}

	return ranks, nil
}

func recommendFollowsGlobal(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	source string) (models.PagerankMap, error) {

	var avoid []uint32 // a slice of nodeIDs that should not be recommended, like self, follows, mutes...
	node, err := DB.NodeByKey(ctx, source)

	switch {
	case errors.Is(err, models.ErrNodeNotFoundDB):
		// do nothing, as we can still recommend.

	case err != nil:
		// if the error is different than node-not-found it means there are issue with our DB, so it's better to fail.
		return nil, fmt.Errorf("failed to fetch the ID of source: %w", err)

	case node == nil:
		// if there is no error and the node is nil, it means there are issue with our DB, so it's better to fail.
		return nil, fmt.Errorf("failed to fetch the ID of source: node is nil")

	default:
		follows, err := DB.Follows(ctx, node.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch follows of %s", source)
		}
		// remove follows and self from the recommendations
		avoid = append(follows[0], node.ID)
	}

	// this should be faster than using DB.AllNodes(). It might happen that some nodeIDs
	// are not associated with any node, but this is not a problem since their pagerank will be 0.
	size := DB.Size(ctx)
	candidates := make([]uint32, size)
	for i := 0; i < size; i++ {
		candidates[i] = uint32(i)
	}

	ranks, err := pagerank.Global(ctx, RWS, candidates...)
	if err != nil {
		return nil, fmt.Errorf("failed to recommend with globalPagerank pagerank: %w", err)
	}

	for _, ID := range avoid {
		delete(ranks, ID)
	}

	return ranks, nil
}

func recommendFollowsPersonalized(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	source string,
	limit int) (models.PagerankMap, error) {

	var avoid []uint32 // a slice of nodeIDs that should not be recommended, like self, follows, mutes...
	node, err := DB.NodeByKey(ctx, source)

	switch {
	case err != nil:
		return nil, fmt.Errorf("failed to fetch the ID of source: %w", err)

	case node == nil:
		return nil, fmt.Errorf("failed to fetch the ID of source: node is nil")

	default:
		follows, err := DB.Follows(ctx, node.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch follows of %s", source)
		}
		// remove follows and self from the recommendations
		avoid = append(follows[0], node.ID)
	}

	ranks, err := pagerank.Personalized(ctx, DB, RWS, node.ID, uint16(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to recommend with personalizedPagerank pagerank: %w", err)
	}

	for _, ID := range avoid {
		delete(ranks, ID)
	}

	return ranks, nil
}

func validateVerifyReputation(args *Args) error {
	if args == nil {
		return ErrNilArgs
	}

	if len(args.Targets) != 1 {
		return fmt.Errorf("%w: exactly one target must be provided for VerifyReputation", ErrInvalidTargets)
	}

	if args.Limit < 1 {
		return fmt.Errorf("%w: limit must be greater than one", ErrInvalidLimit)
	}

	return nil
}

func validateRecommendFollows(args *Args) error {
	if args == nil {
		return ErrNilArgs
	}

	if args.Limit < 1 {
		return fmt.Errorf("%w: limit must be  greater than one", ErrInvalidLimit)
	}

	return nil
}

func validateSortAuthors(args *Args) error {
	if args == nil {
		return ErrNilArgs
	}

	if len(args.Targets) < 1 {
		return fmt.Errorf("%w: at least one target must be provided for SortAuthors", ErrInvalidTargets)
	}

	if args.Limit < 1 {
		return fmt.Errorf("%w: limit must be greater than one", ErrInvalidLimit)
	}

	return nil
}

func validateSearchAuthors(args *Args) error {
	if args == nil {
		return ErrNilArgs
	}

	if len(args.Search) == 0 {
		return fmt.Errorf("%w: the search parameter should not be empty for SearchAuthors", ErrInvalidSearch)
	}

	if len(args.Targets) != 0 {
		return fmt.Errorf("%w: no targets need to be provided for SearchAuthors", ErrInvalidTargets)
	}

	if args.Limit < 1 {
		return fmt.Errorf("%w: limit must be greater than one", ErrInvalidLimit)
	}

	return nil
}

// buildResponse() replaces IDs with pubkeys and returns RankResponses
func buildResponse(ctx context.Context, DB models.Database, nodeRanks pairs) (RankResponses, error) {
	if len(nodeRanks) < 1 {
		// if nodeRanks is empty, we return an empty response. This can happen for example when calling
		// SortAuthors and all args.Targets are not present in our DB.
		return RankResponses{}, nil
	}

	IDs, ranks := nodeRanks.Unpack()
	pubkeys, err := DB.Pubkeys(ctx, IDs...)
	if err != nil {
		return nil, fmt.Errorf("%w: buildResponse: failed to convert nodeIDs to pubkeys: %w", ErrComputationFailed, err)
	}

	res := make(RankResponses, len(nodeRanks))
	for i, pk := range pubkeys {
		if pk == nil {
			return nil, fmt.Errorf("%w: buildResponse: %w ID=%d", ErrComputationFailed, models.ErrNodeNotFoundDB, IDs[i])
		}

		res[i] = RankResponse{Pubkey: *pk, Rank: ranks[i]}
	}

	return res, nil
}

// -----------------------------------HELPERS-----------------------------------

type pair struct {
	ID   uint32
	rank float64
}

type pairs []pair

func (p pairs) Len() int           { return len(p) }
func (p pairs) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p pairs) Less(i, j int) bool { return p[i].rank > p[j].rank }
func (p pairs) Min() (int, float64) {
	if len(p) < 1 {
		panic("dvm.Min: pairs is empty")
	}
	index, min := 0, p[0].rank
	for i, pair := range p {
		if pair.rank < min {
			index = i
			min = pair.rank
		}
	}
	return index, min
}

func (p pairs) Unpack() ([]uint32, []float64) {
	IDs := make([]uint32, len(p))
	ranks := make([]float64, len(p))

	for i, pair := range p {
		IDs[i] = pair.ID
		ranks[i] = pair.rank
	}

	return IDs, ranks
}

// topPairs() returns a slice of `limit` (key, value), sorted by value. Worst case time complexity is O(len(m) * limit)
func topPairs(m models.PagerankMap, limit int) pairs {
	l := min(limit, len(m))
	if l < 1 {
		return nil
	}

	var i int
	var min float64
	pairs := make(pairs, 0, l)

	for ID, rank := range m {
		if len(pairs) < l {
			// append the first l pairs
			pairs = append(pairs, pair{ID: ID, rank: rank})
			i, min = pairs.Min()
			continue
		}

		if rank > min {
			// swap out the smallest pair with the new one
			pairs[i] = pair{ID: ID, rank: rank}
			i, min = pairs.Min()
		}
	}

	sort.Sort(pairs)
	return pairs
}
