package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "github.com/joho/godotenv/autoload"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/relay/pkg/rate"
)

type Config struct {
	RelayConfig
	Limits rate.PagerankRefillPolicy
}

// New returns a configuration structure with default paramenters.
func New() Config {
	return Config{
		RelayConfig: NewRelayConfig(),
		Limits:      rate.NewPagerankRefillPolicy(),
	}
}

func (c Config) Print() {
	c.RelayConfig.Print()
	c.Limits.Print()
}

type RelayConfig struct {
	RelayAddress string
	RedisAddress string
	SQLitePath   string

	Secret string
	Public string
}

// NewRelayConfig returns a relay configuration structure with default paramenters.
func NewRelayConfig() RelayConfig {
	return RelayConfig{
		RelayAddress: "localhost:3334",
		RedisAddress: "localhost:6379",
		SQLitePath:   "relay.sqlite",
	}
}

func (c RelayConfig) Print() {
	fmt.Println("Relay Config:")
	fmt.Printf("  Secret: %s\n", c.Secret)
	fmt.Printf("  Public: %s\n", c.Public)
	fmt.Printf("  RedisAddress: %s\n", c.RedisAddress)
	fmt.Printf("  RelayAddress: %s\n", c.RelayAddress)
	fmt.Printf("  SQLitePath: %s\n", c.SQLitePath)
}

// Load read the variables from the enviroment and parses them into a [Config] struct.
func Load() (*Config, error) {
	var config = New()
	var err error

	for _, item := range os.Environ() {
		keyVal := strings.SplitN(item, "=", 2)
		key, val := keyVal[0], keyVal[1]

		switch key {
		case "SECRET_KEY":
			pubkey, err := nostr.GetPublicKey(val)
			if err != nil {
				return nil, fmt.Errorf("failed to get pubkey from %v: %w", keyVal, err)
			}

			config.RelayConfig.Secret = val
			config.RelayConfig.Public = pubkey

		case "REDIS_ADDRESS":
			config.RelayConfig.RedisAddress = val

		case "RELAY_ADDRESS":
			config.RelayConfig.RelayAddress = val

		case "SQLITE_PATH":
			config.RelayConfig.SQLitePath = val

		case "REFILL_TOKENS":
			config.Limits.RefillTokens, err = strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %v: %w", keyVal, err)
			}

		case "REFILL_INTERVAL_SECONDS":
			config.Limits.RefillIntervalSeconds, err = strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %v: %w", keyVal, err)
			}

		case "MAX_TOKENS_BEFORE_REFILL":
			config.Limits.MaxTokensBeforeRefill, err = strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %v: %w", keyVal, err)
			}

		case "WALKS_THRESHOLD":
			config.Limits.WalksThreshold, err = strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %v: %w", keyVal, err)
			}
		}
	}

	return &config, nil
}
