package ranking

import (
	"errors"
	"fmt"
	"log/slog"
	"math"

	ore "github.com/Open-Ranking/go-sdk"
	"github.com/redis/go-redis/v9"
	"github.com/vertex-lab/crawler_v2/pkg/leaks"
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

const (
	hour     = 3600        // number of seconds in an hour
	infinite = math.MaxInt // for when the TTL is infinite
)

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
