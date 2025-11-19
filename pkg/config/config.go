// The package config defined and loads the configuration parameters from the .env file.
package config

import (
	"fmt"

	_ "github.com/joho/godotenv/autoload"
	"github.com/kelseyhightower/envconfig"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/relay/pkg/core"
	"github.com/vertex-lab/relay/pkg/rate"
)

type Config struct {
	Relay   RelayConfig
	Service core.ServiceConfig
	Refill  rate.RefillPolicy
}

// New returns a config with default paramenters.
func New() Config {
	return Config{
		Relay:   NewRelayConfig(),
		Service: core.NewServiceConfig(),
		Refill:  rate.NewRefillPolicy(),
	}
}

func (c Config) Validate() error {
	if err := c.Relay.Validate(); err != nil {
		return fmt.Errorf("Relay: %w", err)
	}

	if err := c.Refill.Validate(); err != nil {
		return fmt.Errorf("Refill: %w", err)
	}
	return nil
}

type RelayConfig struct {
	Address       string `envconfig:"RELAY_ADDRESS"`
	QueueCapacity int    `envconfig:"QUEUE_CAPACITY"`
	Processors    int    `envconfig:"PROCESSORS"`
	SecretKey     string `envconfig:"SECRET_KEY"`
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
	c.Relay.Print()
	c.Service.Print()
	c.Refill.Print()
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

func (c RelayConfig) Print() {
	fmt.Println("Relay:")
	fmt.Printf("  Address: %s\n", c.Address)
	fmt.Printf("  QueueCapacity: %d\n", c.QueueCapacity)
	fmt.Printf("  Processors: %d\n", c.Processors)
	fmt.Printf("  SecretKey: %s\n", c.SecretKey)
	fmt.Printf("  PublicKey: %s\n", c.PublicKey)
}

// Load creates a new [Config] with default parameters, that get overwritten by
// env variables when specified.
func Load() (Config, error) {
	config := New()
	err := envconfig.Process("", &config)
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
