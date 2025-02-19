// The rate package implemente a simple bucket-rate-limiting with refill.
package rate

import (
	"fmt"
	"sync"
	"time"

	"github.com/puzpuzpuz/xsync/v3"
)

const (
	DefaultFreeTokens int           = 30
	DefaultInterval   time.Duration = 24 * time.Hour
)

type Limiter struct {
	*xsync.MapOf[string, *Bucket]
}

func NewLimiter() Limiter {
	return Limiter{xsync.NewMapOf[string, *Bucket]()}
}

// Bucket contains the number of tokens left for a user, with the last request to implement refilling
type Bucket struct {
	mu      sync.Mutex
	tokens  int
	lastReq time.Time
}

func NewBucket() *Bucket {
	return &Bucket{
		tokens:  DefaultFreeTokens,
		lastReq: time.Now(),
	}
}

// Reject() returns whether the request is rejected based on the cost and the number of tokens in the bucket.
// If it's accepted, it pays the cost.
func (b *Bucket) Reject(cost int) (reject bool, msg string) {
	if b == nil {
		return true, "nil bucket"
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// refill every interval
	if time.Since(b.lastReq) >= DefaultInterval {
		b.tokens = DefaultFreeTokens
	}

	if b.tokens >= cost {
		b.tokens -= cost
		b.lastReq = time.Now()
		return false, ""
	}

	return true, fmt.Sprintf("you've reached the limit of %d free requests per day: DM us if you want unlimited access", DefaultFreeTokens)
}
