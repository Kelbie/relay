package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/relay/pkg/rate"
)

type Config struct {
	RelayConfig
	Limits rate.PagerankRefillPolicy
}

type RelayConfig struct {
	RelayAddress string
	RedisAddress string
	SQLitePath   string

	secret string
	public string
}

// NewConfig() returns a configuration structure with default paramenters.
func NewConfig() Config {
	return Config{
		RelayConfig: NewRelayConfig(),
		Limits:      rate.NewPagerankRefillPolicy(),
	}
}

// NewRelayConfig() returns a relay configuration structure with default paramenters.
func NewRelayConfig() RelayConfig {
	return RelayConfig{
		RelayAddress: "localhost:3334",
		RedisAddress: "localhost:6379",
		SQLitePath:   "relay.sqlite",
	}
}

func (c RelayConfig) Print() {
	fmt.Println("Relay Config:")
	fmt.Printf("  Secret: %s\n", c.secret)
	fmt.Printf("  Public: %s\n", c.public)
	fmt.Printf("  RedisAddress: %s\n", c.RedisAddress)
	fmt.Printf("  RelayAddress: %s\n", c.RelayAddress)
	fmt.Printf("  SQLitePath: %s\n", c.SQLitePath)
}

func (c Config) Print() {
	c.RelayConfig.Print()
	c.Limits.Print()
}

// LoadConfig() read the variables from the enviroment and parses them into a config struct.
func LoadConfig() (*Config, error) {
	var config = NewConfig()
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

			config.RelayConfig.secret = val
			config.RelayConfig.public = pubkey

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
