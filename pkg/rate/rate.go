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

// Pay() tries to deduct `cost` tokens from the bucket of `pubkey` and returns whether the payment was successful.
func (l Limiter) Pay(pubkey string, cost int) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if cost < 0 {
		// cost can be 0 to allow for easier testing. See rate_test.go
		return false, fmt.Errorf("cost cannot be negative: %d", cost)
	}

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

// Bucket() returns the bucket of `pubkey`. If it doesn't exists, it returns an empty bucket.
func (l Limiter) Bucket(pubkey string) (*Bucket, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	key := "creditBucket:" + pubkey
	cmd := l.client.HGetAll(ctx, key)

	var bucket Bucket
	if err := cmd.Scan(&bucket); err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", key, err)
	}

	return &bucket, nil
}

// TopUp() increases the tokens of `pubkey` by `tokens`, and returns the total after the increase.
func (l Limiter) TopUp(pubkey string, tokens int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if tokens < 0 {
		return -1, fmt.Errorf("tokens cannot be negative: %d", tokens)
	}

	key := "creditBucket:" + pubkey
	pipe := l.client.TxPipeline()

	cmd := pipe.HIncrBy(ctx, key, "tokens", int64(tokens))
	pipe.HSet(ctx, key, "last_modified", time.Now().Unix())

	if _, err := pipe.Exec(ctx); err != nil {
		return -1, fmt.Errorf("failed to top-up the balance of %s: %w", key, err)
	}

	return int(cmd.Val()), nil
}
