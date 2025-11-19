// The package core defines the [Service] struct, responsible for handling the
// core business logic of the relay. It defines arguments and the responses
// for each service.
package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/redis/go-redis/v9"
	"github.com/vertex-lab/crawler_v2/pkg/graph"
	"github.com/vertex-lab/crawler_v2/pkg/pagerank"
	"github.com/vertex-lab/crawler_v2/pkg/regraph"
	"github.com/vertex-lab/crawler_v2/pkg/store"
	sqlite "github.com/vertex-lab/nostr-sqlite"
	"github.com/vertex-lab/relay/pkg/credits"
)

var (
	Global       = "globalPagerank"
	Personalized = "personalizedPagerank"
	Followers    = "followerCount"
)

var (
	ErrInvalidSource     = errors.New("invalid source")
	ErrInvalidSort       = errors.New("invalid sort")
	ErrInvalidTarget     = errors.New("invalid target")
	ErrInvalidTargets    = errors.New("invalid targets")
	ErrInvalidLimit      = errors.New("invalid limit")
	ErrInvalidSearch     = errors.New("invalid search")
	ErrBadlyFormattedKey = errors.New("badly formatted key")
	ErrMultipleParams    = errors.New("too many parameters of the same type")

	ErrUnsupportedArgs = errors.New("unsupported args")
	ErrInternal        = errors.New("internal error")
)

// Service encapsulates the business logic of the Vertex services.
type Service struct {
	Sqlite  *sqlite.Store
	Redis   regraph.DB
	Credits credits.Manager
}

type ServiceConfig struct {
	RedisAddress string `envconfig:"REDIS_ADDRESS"`
	SqlitePath   string `envconfig:"SQLITE_PATH"`
	Refill       credits.RefillPolicy
}

// New creates a [Service] initialized with the specified [ServiceConfig].
func NewService(c ServiceConfig) (*Service, error) {
	sqlite, err := store.New(c.SqlitePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize service: %w", err)
	}

	slog.Info("sqlite connected", "address", c.SqlitePath)

	redis, err := regraph.New(&redis.Options{Addr: c.RedisAddress})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize service: %w", err)
	}

	slog.Info("redis connected", "address", c.RedisAddress)

	credits, err := credits.NewManager(redis.Client, c.Refill)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize service: %w", err)
	}

	slog.Info("credits manager initialized")

	return &Service{
		Sqlite:  sqlite,
		Redis:   redis,
		Credits: credits,
	}, nil
}

func NewServiceConfig() ServiceConfig {
	return ServiceConfig{
		RedisAddress: "localhost:6379",
		SqlitePath:   "relay.sqlite",
		Refill:       credits.NewRefillPolicy(),
	}
}

func (c ServiceConfig) Print() {
	fmt.Println("Service Config:")
	fmt.Printf("  Redis Address: %s\n", c.RedisAddress)
	fmt.Printf("  Sqlite Path: %s\n", c.SqlitePath)
	c.Refill.Print()
}

func (c ServiceConfig) Validate() error {
	return c.Refill.Validate()
}

// Close closes the service database connections, releasing resources.
func (s *Service) Close() error {
	err1 := s.Sqlite.Close()
	err2 := s.Redis.Close()

	if err1 == nil && err2 == nil {
		return nil
	}
	return fmt.Errorf("service failed to close: sqlite: %w; redis: %w", err1, err2)
}

// Args represent the arguments for a service endpoint.
type Args interface {
	// Normalize the args in place. It returns an error if invalid.
	Normalize() error

	// Cost returns the cost (measured in credits) of a service call with the provided arguments.
	Cost() int
}

// Allow returns whether the pubkey is allowed to ask for a job with the
// provided args. It's allowed if and only if error is nil.
func (s *Service) Allow(pubkey string, args Args) error {
	err := s.Credits.Deduct(pubkey, args.Cost())
	if err != nil {
		if !errors.Is(err, credits.ErrInsufficientCredits) {
			slog.Error("service.Allow: unexpected error", "error", err)
		}
		return err
	}
	return nil
}

type Algorithm struct {
	Sort   string
	Source string
}

// RankPubkeys ranks the pubkeys according to the provided [Algorithm].
// If a pubkey is not found, the rank is always assumed to be 0.
func (s *Service) rankPubkeys(ctx context.Context, pubkeys []string, algo Algorithm) ([]float64, error) {
	nodes, err := s.Redis.NodeIDs(ctx, pubkeys...)
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
		counts, err := s.Redis.FollowerCounts(ctx, nodes...)
		if err != nil {
			return nil, err
		}

		ranks := make([]float64, len(counts))
		for i, count := range counts {
			ranks[i] = float64(count)
		}
		return ranks, nil

	case Global:
		return pagerank.Global(ctx, s.Redis, nodes...)

	case Personalized:
		source, err := s.Redis.NodeByKey(ctx, algo.Source)
		if err != nil {
			return nil, err
		}

		return pagerank.PersonalizedWithTargets(ctx, s.Redis, source.ID, nodes, 100_000)

	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidSort, algo.Sort)
	}
}

type Profile struct {
	Pubkey string  `json:"pubkey"`
	Rank   float64 `json:"rank"`
}

type DetailedProfile struct {
	Pubkey    string  `json:"pubkey"`
	Rank      float64 `json:"rank"`
	Follows   int     `json:"follows"`
	Followers int     `json:"followers"`
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
