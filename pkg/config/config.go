package config

import (
	"errors"
	"fmt"

	_ "github.com/joho/godotenv/autoload"
	"github.com/kelseyhightower/envconfig"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/relay/pkg/rate"
)

type Config struct {
	RelayConfig
	Refill rate.RefillPolicy
}

// New returns a config with default paramenters.
func New() Config {
	return Config{
		RelayConfig: NewRelayConfig(),
		Refill:      rate.NewRefillPolicy(),
	}
}

func (c Config) Validate() error {
	if err := c.RelayConfig.Validate(); err != nil {
		return fmt.Errorf("Relay: %w", err)
	}

	if err := c.Refill.Validate(); err != nil {
		return fmt.Errorf("Refill: %w", err)
	}
	return nil
}

func (c Config) Print() {
	c.RelayConfig.Print()
	c.Refill.Print()
}

type RelayConfig struct {
	RelayAddress string `envconfig:"RELAY_ADDRESS"`
	RedisAddress string `envconfig:"REDIS_ADDRESS"`
	SQLiteURL    string `envconfig:"SQLITE_URL"`

	SecretKey string `envconfig:"SECRET_KEY"`
	PublicKey string
}

// NewRelayConfig returns a relay configuration structure with default paramenters.
func NewRelayConfig() RelayConfig {
	return RelayConfig{
		RelayAddress: "localhost:3334",
		RedisAddress: "localhost:6379",
		SQLiteURL:    "relay.sqlite",
	}
}

func (c RelayConfig) Validate() error {
	pk, err := nostr.GetPublicKey(c.SecretKey)
	if err != nil {
		return fmt.Errorf("secret key is not a valid: %w", err)
	}

	if pk != c.PublicKey {
		return errors.New("secret and public keys don't match")
	}
	return nil
}

func (c RelayConfig) Print() {
	fmt.Println("Relay:")
	fmt.Printf("  SecretKey: %s\n", c.SecretKey)
	fmt.Printf("  PublicKey: %s\n", c.PublicKey)
	fmt.Printf("  RedisAddress: %s\n", c.RedisAddress)
	fmt.Printf("  RelayAddress: %s\n", c.RelayAddress)
	fmt.Printf("  SQLiteURL: %s\n", c.SQLiteURL)
}

// Load creates a new [Config] with default parameters.
// Then, if the corresponding environment variable is set, it overwrites them.
func Load() (Config, error) {
	config := New()

	err := envconfig.Process("", &config)
	if err != nil {
		return Config{}, fmt.Errorf("config.Load: %w", err)
	}

	config.PublicKey, err = nostr.GetPublicKey(config.SecretKey)
	if err != nil {
		return Config{}, fmt.Errorf("secret key is not a valid: %w", err)
	}

	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("config.Load: %w", err)
	}

	return config, nil
}
