// The package rate provides a concurrent, memory-efficient implementation
// of the Token Bucket algorithm tailored for per-entity rate limiting.
//
// Each entity is identified by a string key and possesses its own dedicated,
// stateful token bucket, that is refilled according to the logic specified by the [Refiller].
package rate

import (
	"sync"
	"time"
)

// Limiter is a memory-efficient implementation of the Token Bucket algorithm
// tailored for per-entity rate limiting. Entities (e.g. API keys, IP addresses ...)
// are represented as strings.
type Limiter struct {
	mu       sync.RWMutex
	buckets  map[string]*Bucket
	refiller Refiller
}

type Bucket struct {
	mu         sync.Mutex
	Tokens     float64
	LastRefill time.Time
}

// Refiller is an interface that must be implemented by the refill policies of the limiter.
type Refiller interface {
	// NewBucket creates a fully initialized Bucket object for a new entity.
	NewBucket(entity string) *Bucket

	// Refill updates the given existing bucket's token count.
	Refill(*Bucket) error
}

// DefaultRefiller is the default refill policy that every `Interval`,
// refills `TokensPerInterval` without exceeding the `MaxTokens`.
// A new Bucket is initialized
type DefaultRefiller struct {
	InitialTokens     float64
	MaxTokens         float64
	TokensPerInterval float64
	Interval          time.Duration
}

func (r DefaultRefiller) NewBucket(_ string) *Bucket {
	return &Bucket{
		Tokens:     r.InitialTokens,
		LastRefill: time.Now(),
	}
}

func (r DefaultRefiller) Refill(b *Bucket) error {
	if r.Interval <= 0 {
		return nil
	}

	if b.Tokens >= r.MaxTokens {
		// if a bucket has more than the maximum tokens,
		// don't continue as that would decrease its tokens.
		return nil
	}

	refills := time.Since(b.LastRefill) / r.Interval
	if refills == 0 {
		return nil
	}

	b.Tokens = min(r.MaxTokens, b.Tokens+float64(refills)*r.TokensPerInterval)
	b.LastRefill = b.LastRefill.Add(refills * r.Interval)
	return nil
}

func NewLimiter(r Refiller) *Limiter {
	return &Limiter{
		buckets:  make(map[string]*Bucket, 1000),
		refiller: r,
	}
}

func (l *Limiter) Reject(entity string, cost float64) bool {
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

	l.refiller.Refill(bucket)
	if bucket.Tokens < cost {
		return true
	}
	bucket.Tokens -= cost
	return false
}
