package api

import (
	"fmt"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/relay/pkg/core"
	"github.com/vertex-lab/relay/pkg/rate"
)

func NewHandler(config Config, service *core.Service, limiter *rate.Limiter) Handler {
	return Handler{
		service:   service,
		limiter:   limiter,
		secretKey: config.SecretKey,
		stats:     stats{logEvery: config.LogEvery},
	}
}

type Config struct {
	SecretKey string `env:"API_SECRET_KEY"`
	LogEvery  uint32 `env:"API_LOG_EVERY"`
}

// NewConfig returns an API configuration structure with default paramenters.
func NewConfig() Config {
	return Config{
		LogEvery: 1000,
	}
}

func (c Config) Validate() error {
	if c.LogEvery == 0 {
		return fmt.Errorf("log every must be positive: %d", c.LogEvery)
	}

	_, err := nostr.GetPublicKey(c.SecretKey)
	if err != nil {
		return fmt.Errorf("secret key is invalid: %w", err)
	}
	return nil
}

func (c Config) String() string {
	return fmt.Sprintf(
		"API Config:\n"+
			"\tLogEvery: %d\n"+
			"\tSecretKey: %s\n",
		c.LogEvery,
		c.SecretKey,
	)
}
