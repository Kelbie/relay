package rate

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/redis/go-redis/v9"
)

const (
	DefaultAmount        int           = 100
	DefaultInterval      time.Duration = 24 * time.Hour
	DefaultWalkThreshold int           = 5
)

var NoRefill = RefillPolicy{Amount: 0}

type Bucket struct {
	Tokens       int   `redis:"tokens"`
	LastModified int64 `redis:"last_modified"` // unix time
}

// ToEvent returns the bucket as an unsigned kind 22243 nostr event
func (b Bucket) ToEvent() nostr.Event {
	return nostr.Event{
		Kind:      22243,
		CreatedAt: nostr.Now(),
		Tags: nostr.Tags{
			{"credits", strconv.Itoa(b.Tokens)},
			{"lastRequest", strconv.FormatInt(b.LastModified, 10)},
		},
	}
}

type RefillPolicy struct {
	Amount        int           `envconfig:"REFILL_AMOUNT"`
	Interval      time.Duration `envconfig:"REFILL_INTERVAL"`
	WalkThreshold int           `envconfig:"REFILL_WALK_THRESHOLD"`
}

func NewRefillPolicy() RefillPolicy {
	return RefillPolicy{
		Amount:        DefaultAmount,
		Interval:      DefaultInterval,
		WalkThreshold: DefaultWalkThreshold,
	}
}

func (p RefillPolicy) Validate() error {
	if p.Amount < 0 {
		return errors.New("amount cannot be negative")
	}

	if p.WalkThreshold < 0 {
		return errors.New("walk threshold cannot be negative")
	}
	return nil
}

func (p RefillPolicy) Print() {
	fmt.Println("Refill Policy:")
	fmt.Printf("  Amount: %d\n", p.Amount)
	fmt.Printf("  Interval: %v\n", p.Interval)
	fmt.Printf("  Walk Threshold: %d\n", p.WalkThreshold)
}

type Limiter struct {
	client *redis.Client
	refill RefillPolicy
}

// NewLimiter returns a limiter with the specified [RefillPolicy].
func NewLimiter(client *redis.Client, refill RefillPolicy) (Limiter, error) {
	limiter := Limiter{client: client, refill: refill}
	if err := limiter.init(); err != nil {
		return Limiter{}, fmt.Errorf("NewLimiter: %w", err)
	}
	return limiter, nil
}

// init loads the Lua "rate.lua" script from the same directory as this source file
// and registers (or replaces) it as a Redis function.
func (l Limiter) init() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return errors.New("cannot determine caller directory")
	}

	dir := filepath.Dir(filename)
	path := filepath.Join(dir, "rate.lua")
	code, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	err = l.client.FunctionLoadReplace(ctx, string(code)).Err()
	if err != nil {
		return fmt.Errorf("failed to load redis function: %w", err)
	}
	return nil
}

var (
	KeyCreditBucket = "creditBucket:"
	KeyTokens       = "tokens"
	KeyLastModified = "last_modified"
	KeyNotAllowed   = "not allowed"
	KeyAllowed      = "allowed"
)

func creditBucket(pubkey string) string { return KeyCreditBucket + pubkey }

// Allow tries to deduct the cost from the pubkey's tokens and reports whether it suceeded.
func (l Limiter) Allow(pubkey string, cost int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if cost < 0 {
		log.Printf("limiter.Allow: cost cannot be negative: %d", cost)
		return false
	}

	args := []any{
		pubkey,
		cost,
		l.refill.Amount,
		l.refill.Interval.Seconds(),
		l.refill.WalkThreshold,
	}

	result, err := l.client.FCall(ctx, "allow", nil, args...).Result()
	if err != nil {
		log.Printf("limiter.Allow: %v", err)
		return false
	}

	status, ok := result.(string)
	if !ok {
		log.Printf("limiter.Allow: failed to type assert the result as a string: %v", result)
		return false
	}

	switch status {
	case KeyAllowed:
		return true

	case KeyNotAllowed:
		return false

	default:
		log.Printf("limiter.Allow: unexpected status: %s", status)
		return false
	}
}

// Bucket returns the bucket of `pubkey`. If it doesn't exists, it returns an empty bucket.
func (l Limiter) Bucket(pubkey string) (Bucket, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var bucket Bucket
	cmd := l.client.HGetAll(ctx, creditBucket(pubkey))
	if err := cmd.Scan(&bucket); err != nil {
		return Bucket{}, fmt.Errorf("failed to fetch bucket of %s: %w", pubkey, err)
	}

	return bucket, nil
}

// TopUp the tokens of pubkey by amount, and return the total after the increase.
func (l Limiter) TopUp(pubkey string, amount int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if amount < 0 {
		return -1, fmt.Errorf("amount cannot be negative: %d", amount)
	}

	pipe := l.client.TxPipeline()
	pipe.HSet(ctx, creditBucket(pubkey), KeyLastModified, time.Now().Unix())
	cmd := pipe.HIncrBy(ctx, creditBucket(pubkey), KeyTokens, int64(amount))

	if _, err := pipe.Exec(ctx); err != nil {
		return -1, fmt.Errorf("failed to top-up the credits of %s: %w", pubkey, err)
	}

	return int(cmd.Val()), nil
}
