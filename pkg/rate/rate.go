// The package rate provides a concurrent implementation
// of the Token Bucket algorithm tailored for per-entity rate limiting.
//
// Each entity is identified by a string key and possesses its own dedicated,
// stateful token bucket, that is refilled according to the logic specified by the [Refiller].
package rate

import (
	"log/slog"
	"sync"
	"time"
)

// Limiter is an per-entity implementation of the Token Bucket algorithm.
// Entities (e.g. API keys, IP addresses ...) are represented as strings.
type Limiter struct {
	mu       sync.RWMutex
	buckets  map[string]*Bucket
	refiller Refiller
}

type Bucket struct {
	mu         sync.Mutex
	Tokens     float32
	LastRefill time.Time
}

// Refiller is an interface that must be implemented by the refill policies of the limiter.
type Refiller interface {
	// NewBucket creates a fully initialized Bucket object for a new entity.
	NewBucket(entity string) *Bucket

	// Refill updates the entity's bucket.
	Refill(entity string, bucket *Bucket) error
}

func NewLimiter(r Refiller) *Limiter {
	return &Limiter{
		buckets:  make(map[string]*Bucket, 1000),
		refiller: r,
	}
}

// Reject returns true if the entity cannot pay the cost, false if it can.
func (l *Limiter) Reject(entity string, cost float32) bool {
	if cost <= 0 {
		return false
	}

	l.mu.RLock()
	bucket, exists := l.buckets[entity]
	l.mu.RUnlock()

	if !exists {
		l.mu.Lock()
		// re-check to avoid race conditions where the same entity
		// is assigned a bucket multiple times
		bucket, exists = l.buckets[entity]
		if !exists {
			bucket = l.refiller.NewBucket(entity)
			l.buckets[entity] = bucket
		}
		l.mu.Unlock()
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	if err := l.refiller.Refill(entity, bucket); err != nil {
		slog.Error("failed to refill", "error", err, "entity", entity)
		return false
	}

	if bucket.Tokens < cost {
		return true
	}
	bucket.Tokens -= cost
	return false
}

// FlatRefiller applies the same refill policy to every bucket.
// Every `Interval`, it refills `TokensPerInterval` without exceeding the `MaxTokens`.
type FlatRefiller struct {
	InitialTokens     float32       `env:"INITIAL_TOKENS"`
	MaxTokens         float32       `env:"MAX_TOKENS"`
	TokensPerInterval float32       `env:"TOKENS_PER_INTERVAL"`
	Interval          time.Duration `env:"INTERVAL"`
}

func (r FlatRefiller) NewBucket(_ string) *Bucket {
	return &Bucket{
		Tokens:     r.InitialTokens,
		LastRefill: time.Now(),
	}
}

func (r FlatRefiller) Refill(_ string, b *Bucket) error {
	if r.Interval <= 0 {
		return nil
	}

	refills := time.Since(b.LastRefill) / r.Interval
	if refills == 0 {
		return nil
	}

	b.Tokens = min(r.MaxTokens, b.Tokens+float32(refills)*r.TokensPerInterval)
	b.LastRefill = b.LastRefill.Add(refills * r.Interval)
	return nil
}

// DynamicRefiller applies different refills based on the entity.
type DynamicRefiller struct {
	Resolve func(entity string) Refiller
}

func (r DynamicRefiller) NewBucket(e string) *Bucket {
	return r.Resolve(e).NewBucket(e)
}

func (r DynamicRefiller) Refill(e string, b *Bucket) error {
	return r.Resolve(e).Refill(e, b)
}
