package ranking

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"

	ore "github.com/Open-Ranking/go-sdk"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/redis/go-redis/v9"
	"github.com/vertex-lab/crawler_v2/pkg/leaks"
	"github.com/vertex-lab/crawler_v2/pkg/regraph"
	"github.com/vertex-lab/crawler_v2/pkg/store"
	sqlite "github.com/vertex-lab/nostr-sqlite"
)

var (
	ErrUnsupportedAlgo   = errors.New("unsupported algorithm")
	ErrBadlyFormattedKey = errors.New("badly formatted key")
	ErrUnknownPubkey     = errors.New("pubkey is unknown")
)

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
	minute   = 60
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

// NpubToHex tries to convert an npub to an hex pubkey.
func NpubToHex(key string) (string, error) {
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
