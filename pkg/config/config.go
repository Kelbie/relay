// The package config defined and loads the configuration parameters from the .env file.
package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	_ "github.com/joho/godotenv/autoload"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/relay/pkg/core"
	"github.com/vertex-lab/relay/pkg/rate"
	"github.com/vertex-lab/relay/pkg/relay"
)

type Config struct {
	Service core.ServiceConfig
	Limiter rate.LimiterConfig
	Relay   relay.Config
}

// New returns a config with default paramenters.
func New() Config {
	return Config{
		Service: core.NewServiceConfig(),
		Limiter: rate.NewConfig(),
		Relay:   relay.NewConfig(),
	}
}

func (c Config) Validate() error {
	if err := c.Relay.Validate(); err != nil {
		return fmt.Errorf("Relay: %w", err)
	}
	if err := c.Limiter.Validate(); err != nil {
		return fmt.Errorf("Limiter: %w", err)
	}
	if err := c.Service.Validate(); err != nil {
		return fmt.Errorf("Refill: %w", err)
	}
	return nil
}

func (c Config) Print() {
	fmt.Println(c.Service)
	fmt.Println(c.Limiter)
	fmt.Println(c.Relay)
}

// Load creates a new [Config] with default parameters, that get overwritten by
// env variables when specified.
func Load() (Config, error) {
	config := New()
	err := env.Parse(&config)
	if err != nil {
		return Config{}, fmt.Errorf("config.Load: %w", err)
	}

	config.Relay.PublicKey, err = nostr.GetPublicKey(config.Relay.SecretKey)
	if err != nil {
		return Config{}, fmt.Errorf("secret key is invalid: %w", err)
	}

	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("config.Load: %w", err)
	}
	return config, nil
}
