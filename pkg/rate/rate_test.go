package rate

import (
	"sync"
	"testing"
	"time"
)

func TestReject(t *testing.T) {
	refiller := FlatRefiller{
		InitialTokens:     100,
		MaxTokens:         100,
		TokensPerInterval: 0,
	}

	limiter := NewLimiter(refiller)
	entity := "lewis"
	accepted := 0

	for {
		if limiter.Reject(entity, 1) {
			break
		}
		accepted++
	}

	if accepted != 100 {
		t.Fatalf("lewis should have been accepted exactly 100 times")
	}
}

// Run this test with go test --race
func TestConcurrency(t *testing.T) {
	refiller := FlatRefiller{
		InitialTokens:     1000,
		MaxTokens:         1000,
		TokensPerInterval: 100,
		Interval:          time.Hour,
	}

	limiter := NewLimiter(refiller)
	entity := "lewis"

	wg := sync.WaitGroup{}
	wg.Add(10_000)

	for range 10_000 {
		go func() {
			defer wg.Done()
			limiter.Reject(entity, 1)
		}()
	}
	wg.Wait()
}

func TestFlatRefill(t *testing.T) {
	refiller := FlatRefiller{
		MaxTokens:         100,
		TokensPerInterval: 10,
		Interval:          time.Hour,
	}

	tests := []struct {
		name     string
		bucket   *Bucket
		expected *Bucket
	}{
		{
			name:     "tokens higher than max (don't decrease)",
			bucket:   &Bucket{Tokens: 10_000},
			expected: &Bucket{Tokens: 10_000},
		},
		{
			name:     "no refill (too soon)",
			bucket:   &Bucket{Tokens: 10, LastRefill: time.Now()},
			expected: &Bucket{Tokens: 10, LastRefill: time.Now()},
		},
		{
			name:     "refill",
			bucket:   &Bucket{Tokens: 10, LastRefill: time.Now().Add(-2 * time.Hour)},
			expected: &Bucket{Tokens: 30, LastRefill: time.Now()},
		},
		{
			name:     "full refill",
			bucket:   &Bucket{Tokens: 10, LastRefill: time.Now().Add(-24 * time.Hour)},
			expected: &Bucket{Tokens: 100, LastRefill: time.Now()},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			refiller.Refill("", test.bucket)

			if test.bucket.Tokens != test.expected.Tokens {
				t.Fatalf("expected tokens %v, got %v", test.expected.Tokens, test.bucket.Tokens)
			}

			if test.expected.LastRefill.Sub(test.bucket.LastRefill) > time.Millisecond {
				t.Fatalf("expected last refill %v, got %v", test.expected.LastRefill, test.bucket.LastRefill)
			}
		})
	}
}
