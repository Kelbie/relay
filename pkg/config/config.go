// The package config defined and loads the configuration parameters from the .env file.
package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	_ "github.com/joho/godotenv/autoload"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/relay/pkg/core"
	"github.com/vertex-lab/relay/pkg/rate"
)

type Config struct {
	Relay   RelayConfig
	Service core.ServiceConfig
	Limiter rate.LimiterConfig
}

// New returns a config with default paramenters.
func New() Config {
	return Config{
		Relay:   NewRelayConfig(),
		Service: core.NewServiceConfig(),
		Limiter: rate.NewConfig(),
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

type RelayConfig struct {
	Address       string `env:"RELAY_ADDRESS"`
	Domain        string `env:"RELAY_DOMAIN"` // the domain of the server/relay, used for nip-42
	QueueCapacity int    `env:"QUEUE_CAPACITY"`
	Processors    int    `env:"PROCESSORS"`
	SecretKey     string `env:"SECRET_KEY"`
	PublicKey     string ``
}

// NewRelayConfig returns a relay configuration structure with default paramenters.
func NewRelayConfig() RelayConfig {
	return RelayConfig{
		Address:       "localhost:3334",
		QueueCapacity: 1000,
		Processors:    4,
	}
}

func (c Config) Print() {
	fmt.Println(c.Relay)
	fmt.Println(c.Service)
	fmt.Println(c.Limiter)
}

func (c RelayConfig) Validate() error {
	if c.QueueCapacity < 0 {
		return fmt.Errorf("queue capacity value must be positiveL %d", c.QueueCapacity)
	}

	if c.Processors < 0 {
		return fmt.Errorf("processors value must be positive: %d", c.Processors)
	}

	pk, err := nostr.GetPublicKey(c.SecretKey)
	if err != nil {
		return fmt.Errorf("secret key is invalid: %w", err)
	}

	if pk != c.PublicKey {
		return fmt.Errorf("secret and public keys don't match")
	}
	return nil
}

func (c RelayConfig) String() string {
	return fmt.Sprintf(
		"Relay Config:\n"+
			"\tAddress: %s\n"+
			"\tQueue Capacity: %d\n"+
			"\tProcessors: %d\n"+
			"\tSecretKey: %s\n"+
			"\tPublicKey: %s\n",
		c.Address,
		c.QueueCapacity,
		c.Processors,
		c.SecretKey,
		c.PublicKey,
	)
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
