package dvm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/vertex-lab/crawler/pkg/models"
	"github.com/vertex-lab/crawler/pkg/pagerank"
	"github.com/vertex-lab/crawler/pkg/utils/sliceutils"
	"github.com/vertex-lab/relay/pkg/eventstore"
	"github.com/vertex-lab/relay/pkg/pairs"
)

type (
	// PubkeyRank is the pair (pubkey, rank). It's the basis for all DVM responses.
	PubkeyRank  = pairs.Pair[string, float64]
	PubkeyRanks = pairs.Pairs[string, float64]

	// NodeRank is the pair (nodeID, rank). Only used for internal computations.
	NodeRank  = pairs.Pair[uint32, float64]
	NodeRanks = pairs.Pairs[uint32, float64]
)

// VerifyReputation() returns the rank of the target and its highest ranked followers.
// All ranks use the specified args.Algorithm.
// For more info read: https://vertexlab.io/docs/nips/verify-reputation-dvm/
func VerifyReputation(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *VerifyReputationArgs) (PubkeyRanks, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	target, err := DB.NodeByKey(ctx, args.Target)
	if errors.Is(err, models.ErrNodeNotFoundDB) {
		// if target is not found in our database, we assume it's a low-reputation key (rank of 0).
		return PubkeyRanks{{Key: args.Target, Val: 0}}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("VerifyReputation %w: failed to fetch the target's ID: %w", ErrInternal, err)
	}

	IDs, err := DB.Followers(ctx, target.ID)
	if err != nil {
		return nil, fmt.Errorf("VerifyReputation %w: failed to fetch the followers of the target: %w", ErrInternal, err)
	}

	followerIDs := IDs[0]
	nodesToRank := append([]uint32{target.ID}, followerIDs...)

	nodeRanks, err := rankNodes(ctx, DB, RWS, nodesToRank, args.Algorithm)
	if err != nil {
		return nil, fmt.Errorf("VerifyReputation %w", err)
	}

	topFollowers := nodeRanks[1:].Top(args.Limit)
	nodeRanks = append(nodeRanks[0:1], topFollowers...) // the response MUST have the target in the first position

	response, err := ToPubkeys(ctx, DB, nodeRanks)
	if err != nil {
		return nil, fmt.Errorf("VerifyReputation %w: %w", ErrInternal, err)
	}

	return response, nil
}

// SortProfiles() returns the rank of each specified target.
// All ranks use the specified args.Algorithm.
// For more info read: https://vertexlab.io/docs/nips/sort-authors-dvm/
func SortProfiles(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *SortProfilesArgs) (PubkeyRanks, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	IDs, err := DB.NodeIDs(ctx, args.Targets...)
	if err != nil {
		return nil, fmt.Errorf("SortProfiles %w: failed to fetch the IDs of the targets: %w", ErrInternal, err)
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

	targetRanks, err := rankNodes(ctx, DB, RWS, targetIDs, args.Algorithm)
	if err != nil {
		return nil, fmt.Errorf("SortProfiles %w: %w", ErrInternal, err)
	}

	_, ranks := targetRanks.Unpack()
	response, err := pairs.Pack(args.Targets, ranks)
	if err != nil {
		return nil, fmt.Errorf("SortProfiles %w: %w", ErrInternal, err)
	}

	return response.Top(args.Limit), nil
}

// SearchProfiles() returns the top ranked pubkeys whose kind:0s contain the provided string.
// All ranks use the specified args.Sort algorithm.
// For more info read:
func SearchProfiles(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	eventStore *eventstore.Store,
	args *SearchProfilesArgs) (PubkeyRanks, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	response, err := searchAuthors(ctx, eventStore, args.Search)
	if err != nil {
		return nil, fmt.Errorf("SearchProfiles: %w", err)
	}

	pubkeys, searchRanks := response.Unpack()
	IDs, err := DB.NodeIDs(ctx, pubkeys...)
	if err != nil {
		return nil, fmt.Errorf("SearchProfiles %w: failed to fetch the IDs of search results: %w", ErrInternal, err)
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

	targetRanks, err := rankNodes(ctx, DB, RWS, targetIDs, args.Algorithm)
	if err != nil {
		return nil, fmt.Errorf("SearchProfiles: %w", err)
	}

	for i, target := range targetRanks {
		// merge ranks and searchRanks in order to give more accurate search results
		response[i].Val = math.Pow(searchRanks[i], 3) * target.Val
	}

	return response.Top(args.Limit), nil
}

// The function searchAuthors() performs full text seach on the profiles (kind:0s) using the specified search term.
// It returns the pubkeys and search scores (positives, higher is better) of the SQL query.
func searchAuthors(
	ctx context.Context,
	eventStore *eventstore.Store,
	search string) (PubkeyRanks, error) {

	if nostr.IsValidPublicKey(search) {
		return pairs.Pairs[string, float64]{{Key: search, Val: 1}}, nil
	}

	if strings.HasPrefix(search, "npub") {
		_, pubkey, err := nip19.Decode(search)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrBadlyFormattedKey, err)
		}

		pk, ok := pubkey.(string)
		if !ok {
			return nil, ErrBadlyFormattedKey
		}

		return pairs.Pairs[string, float64]{{Key: pk, Val: 1}}, nil
	}

	var matches int
	row := eventStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM profiles_fts WHERE profiles_fts MATCH ?", search)
	if err := row.Scan(&matches); err != nil {
		return nil, fmt.Errorf("failed to count matches: %w", err)
	}

	var d, limit = dampening(matches), limit(matches)
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

	pubRanks := make(PubkeyRanks, 0, limit) // pre-allocating
	var pk string
	var rank float64

	for rows.Next() {
		if err = rows.Scan(&pk, &rank); err != nil {
			return nil, fmt.Errorf("failed to scan the results of the query: %w", err)
		}

		// bm25 scores are all negative but we prefer to have positive scores, since we need the top from highest to lowest
		pubRanks = append(pubRanks, PubkeyRank{Key: pk, Val: -rank})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan the results of the query: %w", err)
	}

	return pubRanks, nil
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

// RecommendFollows() uses the specified args.Algorithm to return a list of recommendations for args.Source.
// The recommended pubkeys are the highest ranked, excluding args.Source and its follows (if any).
// For more info read: https://vertexlab.io/docs/nips/recommend-follows-dvm/
func RecommendFollows(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *RecommendFollowsArgs) (PubkeyRanks, error) {

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var nodeRanks NodeRanks
	var err error
	switch args.Sort {
	case Global:
		nodeRanks, err = recommendFollowsGlobal(ctx, DB, RWS, args.Source)

	case Personalized:
		nodeRanks, err = recommendFollowsPersonalized(ctx, DB, RWS, args.Source, 30)

	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidSort, args.Sort)
	}

	if err != nil {
		return nil, fmt.Errorf("RecommendFollows: %w", err)
	}

	topNodes := nodeRanks.Top(args.Limit)
	response, err := ToPubkeys(ctx, DB, topNodes)
	if err != nil {
		return nil, fmt.Errorf("RecommendFollows %w: %w", ErrInternal, err)
	}

	return response, nil
}

func recommendFollowsGlobal(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	source string) (NodeRanks, error) {

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
		// remove follows and self from the recommendations
		avoid = append(follows[0], node.ID)
	}

	// this should be faster than using DB.AllNodes(). It might happen that some nodeIDs
	// are not associated with any node, but this is not a problem since their pagerank will be 0.
	candidates := make([]uint32, DB.Size(ctx))
	for i := 0; i < len(candidates); i++ {
		candidates[i] = uint32(i)
	}

	candidates = sliceutils.Difference(candidates, avoid)
	rankMap, err := pagerank.Global(ctx, RWS, candidates...)
	if err != nil {
		return nil, fmt.Errorf("failed to recommend with globalPagerank pagerank: %w", err)
	}

	nodeRanks := make(NodeRanks, 0, len(rankMap))
	for ID, rank := range rankMap {
		nodeRanks = append(nodeRanks, NodeRank{Key: ID, Val: rank})
	}

	return nodeRanks, nil
}

func recommendFollowsPersonalized(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	source string,
	limit int) (NodeRanks, error) {

	var avoid []uint32 // a slice of nodeIDs that should not be recommended, like self, follows, mutes...
	node, err := DB.NodeByKey(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the ID of source: %w", err)
	}

	follows, err := DB.Follows(ctx, node.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch follows of %s", source)
	}

	// remove follows and self from the recommendations
	avoid = append(follows[0], node.ID)

	pp, err := pagerank.Personalized(ctx, DB, RWS, node.ID, uint16(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to recommend with personalizedPagerank pagerank: %w", err)
	}

	for _, ID := range avoid {
		delete(pp, ID)
	}

	nodeRanks := make(NodeRanks, 0, len(pp))
	for ID, rank := range pp {
		nodeRanks = append(nodeRanks, NodeRank{Key: ID, Val: rank})
	}

	return nodeRanks, nil
}

// rankNodes() associates a rank to each node ID by applying the specified algorithm.
// If the nodeID is not found, the rank is always assumed to be 0.
func rankNodes(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	IDs []uint32,
	algo Algorithm) (NodeRanks, error) {

	if len(IDs) == 0 {
		return nil, nil
	}

	var rankMap models.PagerankMap
	var err error

	switch algo.Sort {
	case Global:
		rankMap, err = pagerank.Global(ctx, RWS, IDs...)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to sort with %s: %w", ErrInternal, algo.Sort, err)
		}

	case Personalized:
		source, err := DB.NodeIDs(ctx, algo.Source)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to sort with %s: %w", ErrInternal, algo.Sort, err)
		}

		if source[0] == nil {
			return nil, fmt.Errorf("%w: pubkey was not found", ErrInvalidSource)
		}

		rankMap, err = pagerank.Personalized(ctx, DB, RWS, *source[0], 100)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to sort with %s: %w", ErrInternal, algo.Sort, err)
		}

	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidSort, algo.Sort)
	}

	nodeRanks := make(NodeRanks, len(IDs))
	for i, ID := range IDs {
		nodeRanks[i] = NodeRank{Key: ID, Val: rankMap[ID]}
	}

	return nodeRanks, nil
}

// -----------------------------------HELPERS-----------------------------------

// ToPubkeys() is used for converting the nodeIDs of Pairs[uint32, float64] into pubkeys.
func ToPubkeys(
	ctx context.Context,
	DB models.Database,
	nodeRanks NodeRanks) (PubkeyRanks, error) {

	IDs, ranks := nodeRanks.Unpack()
	pubkeys, err := DB.Pubkeys(ctx, IDs...)
	if err != nil {
		return nil, err
	}

	pubkeyRanks := make(PubkeyRanks, len(IDs))
	for i, pk := range pubkeys {
		if pk == nil {
			return nil, fmt.Errorf("failed to fetch the pubkey of nodeID %d", IDs[i])
		}

		pubkeyRanks[i] = PubkeyRank{Key: *pk, Val: ranks[i]}
	}

	return pubkeyRanks, nil
}
