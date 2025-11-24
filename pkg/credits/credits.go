// The package credits defines a credit manager that uses the token bucket algorith.
// Each identity (e.g. nostr pubkey) has it's own bucket (potentially empty).
// Buckets might be periodically refilled based on the specified [RefillPolicy].
package credits

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/redis/go-redis/v9"
)

type Bucket struct {
	Tokens       int   `redis:"tokens"`
	LastModified int64 `redis:"last_modified"` // unix time
}

// ToEvent encodes the bucket as an unsigned nostr event.
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

var NoRefill = RefillPolicy{Amount: 0}

func NewRefillPolicy() RefillPolicy {
	return RefillPolicy{
		Amount:        100,
		Interval:      24 * time.Hour,
		WalkThreshold: 5,
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

type Manager struct {
	client *redis.Client
	refill RefillPolicy
}

// NewManager returns a credit manager with the specified [RefillPolicy].
func NewManager(client *redis.Client, refill RefillPolicy) (Manager, error) {
	manager := Manager{client: client, refill: refill}
	if err := manager.init(); err != nil {
		return Manager{}, fmt.Errorf("NewManager: %w", err)
	}
	return manager, nil
}

// init loads the Lua "credits.lua" script from the same directory as this source file
// and registers (or replaces) it as a Redis function.
func (m Manager) init() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return errors.New("cannot determine caller directory")
	}

	dir := filepath.Dir(filename)
	path := filepath.Join(dir, "credits.lua")
	code, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	err = m.client.FunctionLoadReplace(ctx, string(code)).Err()
	if err != nil {
		return fmt.Errorf("failed to load redis function: %w", err)
	}
	return nil
}

var (
	KeyCreditBucket = "creditBucket:"
	KeyTokens       = "tokens"
	KeyLastModified = "last_modified"
	KeySuccess      = "successful deduction"
	KeyFailed       = "failed deduction"

	ErrInsufficientCredits = errors.New("you don't have enough credits to fulfil the request. Send us a DM and we'll give you a top-up for free!")
)

// Deduct tries to deduct the cost from the pubkey's tokens.
// It succeed if and only if the error is nil.
func (m Manager) Deduct(pubkey string, cost int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if cost < 0 {
		return errors.New("cost cannot be negative")
	}

	args := []any{
		pubkey,
		cost,
		m.refill.Amount,
		m.refill.Interval.Seconds(),
		m.refill.WalkThreshold,
	}

	result, err := m.client.FCall(ctx, "deduct", nil, args...).Result()
	if err != nil {
		return fmt.Errorf("failed to call deduct redis function: %w", err)
	}

	status, ok := result.(string)
	if !ok {
		return fmt.Errorf("failed to type assert the result as a string: %s", result)
	}

	switch status {
	case KeyFailed:
		return ErrInsufficientCredits

	case KeySuccess:
		return nil

	default:
		return fmt.Errorf("unexpected status: %s", status)
	}
}

func creditBucket(pubkey string) string { return KeyCreditBucket + pubkey }

// Bucket returns the bucket of `pubkey`. If it doesn't exists, it returns an empty bucket.
func (m Manager) Bucket(pubkey string) (Bucket, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var bucket Bucket
	cmd := m.client.HGetAll(ctx, creditBucket(pubkey))
	if err := cmd.Scan(&bucket); err != nil {
		return Bucket{}, fmt.Errorf("failed to fetch bucket of %s: %w", pubkey, err)
	}
	return bucket, nil
}

// TopUp the tokens of pubkey by amount, and return the total after the increase.
func (m Manager) TopUp(pubkey string, amount int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if amount < 0 {
		return -1, fmt.Errorf("amount cannot be negative: %d", amount)
	}

	pipe := m.client.TxPipeline()
	pipe.HSet(ctx, creditBucket(pubkey), KeyLastModified, time.Now().Unix())
	cmd := pipe.HIncrBy(ctx, creditBucket(pubkey), KeyTokens, int64(amount))

	if _, err := pipe.Exec(ctx); err != nil {
		return -1, fmt.Errorf("failed to top-up the credits of %s: %w", pubkey, err)
	}
	return int(cmd.Val()), nil
}
