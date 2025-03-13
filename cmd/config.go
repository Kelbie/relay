package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/nbd-wtf/go-nostr"
)

type Config struct {
	RedisAddress string
	RelayAddress string
	SQLitePath   string

	secret string
	public string
}

// NewConfig() returns a configuration structure with default paramenters
func NewConfig() Config {
	return Config{
		RedisAddress: "localhost:6379",
		RelayAddress: "localhost:3334",
		SQLitePath:   "relay.sqlite",
	}
}

func (c Config) Print() {
	fmt.Println("Config:")
	fmt.Printf("  Secret: %s\n", c.secret)
	fmt.Printf("  Public: %s\n", c.public)
	fmt.Printf("  RedisAddress: %s\n", c.RedisAddress)
	fmt.Printf("  RelayAddress: %s\n", c.RelayAddress)
	fmt.Printf("  SQLitePath: %s\n", c.SQLitePath)
}

// LoadConfig() read the variables from the enviroment and parses them into a config struct.
func LoadConfig() (*Config, error) {
	config := NewConfig()

	for _, item := range os.Environ() {
		keyVal := strings.SplitN(item, "=", 2)
		key, val := keyVal[0], keyVal[1]

		switch key {
		case "SECRET_KEY":
			pubkey, err := nostr.GetPublicKey(val)
			if err != nil {
				return nil, fmt.Errorf("failed to get pubkey: %w", err)
			}

			config.secret = val
			config.public = pubkey

		case "REDIS_ADDRESS":
			config.RedisAddress = val

		case "RELAY_ADDRESS":
			config.RelayAddress = val

		case "SQLITE_PATH":
			config.SQLitePath = val
		}
	}

	return &config, nil
}
