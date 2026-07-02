package ranking

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	ore "github.com/Open-Ranking/go-sdk"
	"github.com/pippellia-btc/slicex"
	"github.com/redis/go-redis/v9"
	"github.com/vertex-lab/crawler_v2/pkg/graph"
	"github.com/vertex-lab/crawler_v2/pkg/leaks"
	"github.com/vertex-lab/crawler_v2/pkg/pagerank"
	"github.com/vertex-lab/crawler_v2/pkg/regraph"
	"github.com/vertex-lab/crawler_v2/pkg/store"
	sqlite "github.com/vertex-lab/nostr-sqlite"
)

var ErrUnsupportedAlgo = errors.New("unsupported algorithm")

// Service encapsulates the business logic of the Vertex services.
type Service struct {
	Sqlite *sqlite.Store
	Graph  regraph.DB
	Leaks  *leaks.DB
}

// New creates a [Service] initialized with the specified [Config].
func NewService(c Config) (*Service, error) {
	sqlite, err := store.New(c.SqlitePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize service: %w", err)
	}
	slog.Info("sqlite connected", "address", c.SqlitePath)

	redis := redis.NewClient(&redis.Options{Addr: c.RedisAddress})
	leaks := leaks.NewDB(redis)
	graph, err := regraph.New(redis)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize service: %w", err)
	}
	slog.Info("redis connected", "address", c.RedisAddress)

	return &Service{
		Sqlite: sqlite,
		Graph:  graph,
		Leaks:  leaks,
	}, nil
}

// Close closes the service database connections, releasing resources.
func (s *Service) Close() error {
	err1 := s.Sqlite.Close()
	err2 := s.Graph.Close()

	if err1 == nil && err2 == nil {
		return nil
	}
	return fmt.Errorf("service failed to close: sqlite: %w; redis: %w", err1, err2)
}

// Supported open ranking algorithms.
var (
	GlobalPagerank       ore.AlgorithmID = "global-pagerank"
	FollowersCount       ore.AlgorithmID = "followers-count"
	PersonalizedPagerank ore.AlgorithmID = "personalized-pagerank"
)

const hour = 3600 // number of seconds in an hour

// TTL returns the time-to-live (TTL) for a given algorithm, in seconds.
func TTL(algo ore.AlgorithmID) int {
	switch algo {
	case GlobalPagerank:
		return 24 * hour
	case FollowersCount:
		return 12 * hour
	case PersonalizedPagerank:
		return 6 * hour
	default:
		return 0
	}
}

// StatsPubkey returns the stats for a given pubkey, as defined by ORE-02.
// The request is assumed to have been validated by the caller.
// Learn more here: https://github.com/Open-Ranking/protocol/blob/main/02.md
func (s *Service) StatsPubkey(ctx context.Context, r ore.StatsPubkeyRequest) (ore.StatsPubkeyResponse, error) {
	target, err := s.Graph.NodeByKey(ctx, r.Pubkey)
	if errors.Is(err, graph.ErrNodeNotFound) {
		// target is not found, assume it's a low-reputation key
		// with rank of 0 and no follows / followers
		return ore.StatsPubkeyResponse{Pubkey: r.Pubkey, Rank: 0}, nil
	}
	if err != nil {
		return ore.StatsPubkeyResponse{}, err
	}

	var rank float64
	if target.Status != graph.StatusLeaked {
		// if a pubkey has been leaked we show its rank as 0, otherwise we rank it.
		ranks, err := s.rankNodes(ctx, r.Algorithm, r.POV, target.ID)
		if err != nil {
			return ore.StatsPubkeyResponse{}, err
		}
		rank = ranks[0]
	}

	follows, err := s.Graph.FollowCounts(ctx, target.ID)
	if err != nil {
		return ore.StatsPubkeyResponse{}, err
	}

	followers, err := s.Graph.FollowerCounts(ctx, target.ID)
	if err != nil {
		return ore.StatsPubkeyResponse{}, err
	}

	// stats of a pubkey change rather frequently, so I prefer to remain
	// conservative and suggest a TTL of just 1 hours.
	ttl := hour

	res := ore.StatsPubkeyResponse{
		Pubkey:    r.Pubkey,
		Rank:      rank,
		Follows:   &follows[0],
		Followers: &followers[0],
		TTL:       &ttl,
	}
	return res, nil
}

// RankPubkeys returns the rank of a batch of pubkeys, as defined by ORE-03.
// The request is assumed to have been validated by the caller.
// Learn more here: https://github.com/Open-Ranking/protocol/blob/main/03.md
func (s *Service) RankPubkeys(ctx context.Context, r ore.RankPubkeysRequest) (ore.RankPubkeysResponse, error) {
	nodes, err := s.Graph.NodeIDs(ctx, r.Pubkeys...)
	if err != nil {
		return ore.RankPubkeysResponse{}, err
	}
	ranks, err := s.rankNodes(ctx, r.Algorithm, r.POV, nodes...)
	if err != nil {
		return ore.RankPubkeysResponse{}, err
	}

	ranking := slicex.Pack(r.Pubkeys, ranks)
	top := ranking.MaxK(r.Limit)
	ttl := TTL(r.Algorithm)

	res := ore.RankPubkeysResponse{
		Results: make([]ore.RankedPubkey, top.Len()),
		TTL:     &ttl,
	}
	for i, t := range top {
		res.Results[i].Pubkey = t.Key
		res.Results[i].Rank = t.Val
	}
	return res, nil
}

// RecommendPubkeys returns a batch of pubkeys that are recommended, as defined by ORE-04.
// The request is assumed to have been validated by the caller.
// Learn more here: https://github.com/Open-Ranking/protocol/blob/main/04.md
func (s *Service) RecommendPubkeys(ctx context.Context, r ore.RecommendPubkeysRequest) (ore.RecommendPubkeysResponse, error) {
	candidates, err := s.candidates(ctx, r.Algorithm, r.POV)
	if err != nil {
		return ore.RecommendPubkeysResponse{}, err
	}

	nodes, ranks := candidates.MaxK(r.Limit).Unpack()
	pubkeys, err := s.Graph.Pubkeys(ctx, nodes...)
	if err != nil {
		return ore.RecommendPubkeysResponse{}, err
	}

	ttl := TTL(r.Algorithm)
	res := ore.RecommendPubkeysResponse{
		Results: make([]ore.RankedPubkey, len(pubkeys)),
		TTL:     &ttl,
	}
	for i := range pubkeys {
		res.Results[i].Pubkey = pubkeys[i]
		res.Results[i].Rank = ranks[i]
	}
	return res, nil
}

// rankNodes ranks the nodes according to the provided [ore.AlgorithmID].
// If a node is not found, the rank is always assumed to be 0.
func (s *Service) rankNodes(ctx context.Context, algo ore.AlgorithmID, pov string, nodes ...graph.ID) ([]float64, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	switch algo {
	case FollowersCount:
		counts, err := s.Graph.FollowerCounts(ctx, nodes...)
		if err != nil {
			return nil, err
		}

		ranks := make([]float64, len(counts))
		for i, count := range counts {
			ranks[i] = float64(count)
		}
		return ranks, nil

	case GlobalPagerank:
		return pagerank.Global(ctx, s.Graph, nodes...)

	case PersonalizedPagerank:
		source, err := s.Graph.NodeByKey(ctx, pov)
		if err != nil {
			return nil, err
		}
		return pagerank.PersonalizedWithTargets(ctx, s.Graph, source.ID, nodes, 100_000)

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlgo, algo)
	}
}

// nodeRanking is a slice of pairs (nodeID, rank)
type nodeRanking = slicex.Pairs[graph.ID, float64]

func (s *Service) candidates(ctx context.Context, algo ore.AlgorithmID, pov string) (nodeRanking, error) {
	switch algo {
	case GlobalPagerank:
		return s.candidatesGlobal(ctx, pov)
	case PersonalizedPagerank:
		return s.candidatesPersonalized(ctx, pov)
	case FollowersCount:
		return s.candidatesFollowers(ctx, pov)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlgo, algo)
	}
}

func (s *Service) candidatesGlobal(ctx context.Context, source string) (nodeRanking, error) {
	avoid := make([]graph.ID, 0, 100) // pre-allocate

	node, err := s.Graph.NodeByKey(ctx, source)
	if err != nil && !errors.Is(err, graph.ErrNodeNotFound) {
		return nil, err
	}

	if !errors.Is(err, graph.ErrNodeNotFound) {
		// add the node representing the source and its follows to avoid,
		// so they can't get recommended.
		follows, err := s.Graph.Follows(ctx, node.ID)
		if err != nil {
			return nil, err
		}
		avoid = append(follows, node.ID)
	}

	// TODO: the working assumption is that the "first" 50k nodes contain
	// the highest ranked nodes. We should find better heuristics.
	candidates := make([]graph.ID, 50_000)
	for i := range 50_000 {
		candidates[i] = graph.ID(strconv.Itoa(i))
	}

	candidates = slicex.Difference(candidates, avoid)
	ranks, err := pagerank.Global(ctx, s.Graph, candidates...)
	if err != nil {
		return nil, err
	}
	return slicex.Pack(candidates, ranks), nil
}

func (s *Service) candidatesFollowers(ctx context.Context, source string) (nodeRanking, error) {
	avoid := make([]graph.ID, 0, 100) // pre-allocate

	node, err := s.Graph.NodeByKey(ctx, source)
	if err != nil && !errors.Is(err, graph.ErrNodeNotFound) {
		return nil, err
	}

	if !errors.Is(err, graph.ErrNodeNotFound) {
		// add the node representing the source and its follows to avoid,
		// so they can't get recommended.
		follows, err := s.Graph.Follows(ctx, node.ID)
		if err != nil {
			return nil, err
		}
		avoid = append(follows, node.ID)
	}

	// TODO: the working assumption is that the "first" 50k nodes contain
	// the highest ranked nodes. We should find better heuristics.
	candidates := make([]graph.ID, 50_000)
	for i := range 50_000 {
		candidates[i] = graph.ID(strconv.Itoa(i))
	}

	candidates = slicex.Difference(candidates, avoid)
	counts, err := s.Graph.FollowerCounts(ctx, candidates...)
	if err != nil {
		return nil, err
	}

	ranking := make(nodeRanking, len(candidates))
	for i := range candidates {
		ranking[i].Key = candidates[i]
		ranking[i].Val = float64(counts[i])
	}
	return ranking, nil
}

func (s *Service) candidatesPersonalized(ctx context.Context, source string) (nodeRanking, error) {
	node, err := s.Graph.NodeByKey(ctx, source)
	if err != nil {
		return nil, err
	}

	follows, err := s.Graph.Follows(ctx, node.ID)
	if err != nil {
		return nil, err
	}

	pp, err := pagerank.Personalized(ctx, s.Graph, node.ID, 100_000)
	if err != nil {
		return nil, err
	}

	delete(pp, node.ID)
	for _, ID := range follows {
		delete(pp, ID)
	}
	return slicex.ToPairs(pp), nil
}
