package rate

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/redis/go-redis/v9"
)

const (
	DefaultRefillTokens          int = 60
	DefaultRefillIntervalSeconds int = 24 * 60 * 60
	DefaultMaxTokensBeforeRefill int = 250
	DefaultWalksThreshold        int = 110
)

type Bucket struct {
	Tokens       int   `redis:"tokens"`
	LastModified int64 `redis:"last_modified"` // unix time
}

// ToEvent returns the bucket as an unsigned kind 22243 nostr event
func (b *Bucket) ToEvent() nostr.Event {
	return nostr.Event{
		Kind:      22243,
		CreatedAt: nostr.Now(),
		Tags: nostr.Tags{
			{"credits", strconv.Itoa(b.Tokens)},
			{"lastRequest", strconv.FormatInt(b.LastModified, 10)},
		},
	}
}

type PagerankRefillPolicy struct {
	RefillTokens          int
	RefillIntervalSeconds int
	MaxTokensBeforeRefill int
	WalksThreshold        int
}

func (p PagerankRefillPolicy) Print() {
	fmt.Println("Refill Policy:")
	fmt.Printf("  Refill Tokens: %d\n", p.RefillTokens)
	fmt.Printf("  Refill Interval: %ds\n", p.RefillIntervalSeconds)
	fmt.Printf("  Max Tokens Before Refill: %d\n", p.MaxTokensBeforeRefill)
	fmt.Printf("  Walk Threshold: %d\n", p.WalksThreshold)
}

func NewPagerankRefillPolicy() PagerankRefillPolicy {
	return PagerankRefillPolicy{
		RefillTokens:          DefaultRefillTokens,
		RefillIntervalSeconds: DefaultRefillIntervalSeconds,
		MaxTokensBeforeRefill: DefaultMaxTokensBeforeRefill,
		WalksThreshold:        DefaultWalksThreshold,
	}
}

type Limiter struct {
	client *redis.Client
	policy PagerankRefillPolicy
}

// NewLimiter returns a limiter with a default [PagerankRefillPolicy].
func NewLimiter(client *redis.Client) Limiter {
	return NewLimiterWithPolicy(client, NewPagerankRefillPolicy())
}

func NewLimiterWithPolicy(client *redis.Client, policy PagerankRefillPolicy) Limiter {
	return Limiter{
		client: client,
		policy: policy,
	}
}

// Allow tries to deduct the cost from the pubkey's tokens and reports whether it suceeded.
func (l Limiter) Allow(pubkey string, cost int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if cost < 0 {
		// cost can be 0 to allow for easier testing. See rate_test.go
		log.Printf("cost cannot be negative: %d", cost)
		return false
	}

	args := []any{pubkey, cost, l.policy.RefillTokens, l.policy.RefillIntervalSeconds, l.policy.MaxTokensBeforeRefill, l.policy.WalksThreshold}
	res, err := l.client.FCall(ctx, "pay", nil, args...).Result()
	if err != nil {
		log.Printf("Limiter: failed to pay: %v", err)
		return false
	}

	code, ok := res.(string)
	if !ok {
		log.Printf("Limiter: failed to pay: failed to type assert the result as a string: %v", res)
		return false
	}

	return code == "paid"
}

// Bucket returns the bucket of `pubkey`. If it doesn't exists, it returns an empty bucket.
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

// TopUp the tokens of pubkey by amount, and return the total after the increase.
func (l Limiter) TopUp(pubkey string, amount int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if amount < 0 {
		return -1, fmt.Errorf("tokens cannot be negative: %d", amount)
	}

	key := "creditBucket:" + pubkey
	pipe := l.client.TxPipeline()

	cmd := pipe.HIncrBy(ctx, key, "tokens", int64(amount))
	pipe.HSet(ctx, key, "last_modified", time.Now().Unix())

	if _, err := pipe.Exec(ctx); err != nil {
		return -1, fmt.Errorf("failed to top-up the balance of %s: %w", key, err)
	}

	return int(cmd.Val()), nil
}
