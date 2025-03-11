package rate

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	DefaultRefillTokens          int = 39
	DefaultRefillIntervalSeconds int = 24 * 60 * 60
	DefaultMaxTokensBeforeRefill int = 250
	DefaultWalksThreshold        int = 110
)

type Bucket struct {
	Tokens       int   `redis:"tokens"`
	LastModified int64 `redis:"last_modified"` // unix time
}

type PagerankRefillPolicy struct {
	refillTokens          int
	refillIntervalSeconds int
	maxTokensBeforeRefill int
	walksThreshold        int
}

func NewPagerankRefillPolicy() PagerankRefillPolicy {
	return PagerankRefillPolicy{
		refillTokens:          DefaultRefillTokens,
		refillIntervalSeconds: DefaultRefillIntervalSeconds,
		maxTokensBeforeRefill: DefaultMaxTokensBeforeRefill,
		walksThreshold:        DefaultWalksThreshold,
	}
}

type Limiter struct {
	client *redis.Client
	policy PagerankRefillPolicy
}

func NewLimiter(client *redis.Client) Limiter {
	return Limiter{
		client: client,
		policy: NewPagerankRefillPolicy(),
	}
}

func (l Limiter) Pay(pubkey string, cost int) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	args := []any{pubkey, cost, l.policy.refillTokens, l.policy.refillIntervalSeconds, l.policy.maxTokensBeforeRefill, l.policy.walksThreshold}
	res, err := l.client.FCall(ctx, "pay", nil, args...).Result()
	if err != nil {
		return false, fmt.Errorf("failed to pay: %w", err)
	}

	code, ok := res.(string)
	if !ok {
		return false, fmt.Errorf("failed to pay: failed to type assert the result as a string: %v", res)
	}

	return code == "paid", nil
}

func (l Limiter) Bucket(pubkey string) (*Bucket, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	key := "creditBucket:" + pubkey
	cmd := l.client.HGetAll(ctx, key)

	var bucket Bucket
	if err := cmd.Scan(&bucket); err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", key, err)
	}

	return &bucket, nil
}
