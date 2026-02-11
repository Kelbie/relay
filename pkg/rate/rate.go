// Package rate provides a configurable token-bucket rate limiter, which is a wrapper around the
// external github.com/pippellia-btc/rate limiter.
package rate

import "github.com/pippellia-btc/rate"

// Limiter wraps the external github.com/pippellia-btc/rate limiter.
// This package intentionally contains no internal token-bucket/limiter logic;
// it keeps only configuration (see config.go) and delegates limiting to the
// upstream implementation.
type Limiter struct {
	*rate.Limiter[string]
	config Config
}

func NewLimiter(c Config) Limiter {
	l := Limiter{
		Limiter: rate.NewLimiter[string](c),
		config:  c,
	}
	return l
}
