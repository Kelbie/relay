package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/redis/go-redis/v9"
	"github.com/vertex-lab/crawler_v2/pkg/graph"
	"github.com/vertex-lab/crawler_v2/pkg/pagerank"
	"github.com/vertex-lab/crawler_v2/pkg/regraph"
	"github.com/vertex-lab/crawler_v2/pkg/store"
	sqlite "github.com/vertex-lab/nostr-sqlite"
)

var (
	Global       string = "globalPagerank"
	Personalized string = "personalizedPagerank"
	Followers    string = "followerCount"
)

var (
	ErrInvalidSource     error = errors.New("invalid source")
	ErrInvalidSort       error = errors.New("invalid sort")
	ErrInvalidTarget     error = errors.New("invalid target")
	ErrInvalidTargets    error = errors.New("invalid targets")
	ErrInvalidLimit      error = errors.New("invalid limit")
	ErrInvalidSearch     error = errors.New("invalid search")
	ErrBadlyFormattedKey error = errors.New("badly formatted key")
	ErrMultipleParams    error = errors.New("too many parameters of the same type")

	ErrInternal  error = errors.New("internal error")
	ErrNoCredits error = errors.New("you don't have enough credits to fulfil the request. Send us a DM and we'll give you a top-up for free!")
)

// Service encapsulates the business logic of the Vertex services.
type Service struct {
	sqlite    *sqlite.Store
	redis     regraph.DB
	secretKey string
}

type Config struct {
	RedisAddress string `envconfig:"REDIS_ADDRESS"`
	SqlitePath   string `envconfig:"SQLITE_PATH"`
	SecretKey    string `envconfig:"SECRET_KEY"`
}

// New creates a [Service] initialized with the specified [Config].
func New(c Config) (*Service, error) {
	sqlite, err := store.New(c.SqlitePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize service: %w", err)
	}

	redis, err := regraph.New(&redis.Options{Addr: c.RedisAddress})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize service: %w", err)
	}

	return &Service{
		sqlite:    sqlite,
		redis:     redis,
		secretKey: c.SecretKey,
	}, nil
}

// Close closes the service database connections, releasing resources.
func (s *Service) Close() error {
	err1 := s.sqlite.Close()
	err2 := s.redis.Close()

	if err1 == nil && err2 == nil {
		return nil
	}
	return fmt.Errorf("service failed to close: sqlite: %w; redis: %w", err1, err2)
}

type Algorithm struct {
	Sort   string
	Source string
}

// RankPubkeys ranks the pubkeys according to the provided [Algorithm].
// If a pubkey is not found, the rank is always assumed to be 0.
func (s *Service) rankPubkeys(ctx context.Context, pubkeys []string, algo Algorithm) ([]float64, error) {
	nodes, err := s.redis.NodeIDs(ctx, pubkeys...)
	if err != nil {
		return nil, err
	}
	return s.rankNodes(ctx, nodes, algo)
}

// RankNodes ranks the nodes according to the provided [Algorithm].
// If a node is not found, the rank is always assumed to be 0.
func (s *Service) rankNodes(ctx context.Context, nodes []graph.ID, algo Algorithm) ([]float64, error) {
	switch algo.Sort {
	case Followers:
		counts, err := s.redis.FollowerCounts(ctx, nodes...)
		if err != nil {
			return nil, err
		}

		ranks := make([]float64, len(counts))
		for i, count := range counts {
			ranks[i] = float64(count)
		}
		return ranks, nil

	case Global:
		return pagerank.Global(ctx, s.redis, nodes...)

	case Personalized:
		source, err := s.redis.NodeByKey(ctx, algo.Source)
		if err != nil {
			return nil, err
		}

		return pagerank.PersonalizedWithTargets(ctx, s.redis, source.ID, nodes, 100_000)

	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidSort, algo.Sort)
	}
}

type Profile struct {
	Pubkey string
	Rank   float64
}

type DetailedProfile struct {
	Pubkey    string
	Rank      float64
	Follows   int
	Followers int
}

// NpubToHex tries to convert an npub to an hex pubkey.
func NpubToHex(key string) (string, error) {
	key = strings.TrimSpace(key)
	if strings.HasPrefix(key, "npub") {
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
