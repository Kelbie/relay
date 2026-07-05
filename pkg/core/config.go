package core

import (
	"fmt"
	"strings"

	"github.com/vertex-lab/relay/pkg/credits"
)

type Config struct {
	RedisAddress string `env:"REDIS_ADDRESS"`
	SqlitePath   string `env:"SQLITE_PATH"`
	Refill       credits.RefillPolicy

	// CreditsDisabled makes [Service.Allow] always succeed, without deducting credits.
	// Useful for self-hosted deployments that don't use the credit system.
	CreditsDisabled bool `env:"CREDITS_DISABLED"`
}

func NewConfig() Config {
	return Config{
		RedisAddress: "localhost:6379",
		SqlitePath:   "relay.sqlite",
		Refill:       credits.NewRefillPolicy(),
	}
}

func (c Config) String() string {
	return fmt.Sprintf("Service:\n"+
		"\tRedis Address: %s\n"+
		"\tSqlite Path: %s\n"+
		"\t"+strings.ReplaceAll(c.Refill.String(), "\n", "\n\t"),
		c.RedisAddress,
		c.SqlitePath,
	)
}

func (c Config) Validate() error {
	if c.RedisAddress == "" {
		return fmt.Errorf("redis address is required")
	}
	if c.SqlitePath == "" {
		return fmt.Errorf("sqlite path is required")
	}
	if err := c.Refill.Validate(); err != nil {
		return fmt.Errorf("invalid credits config: %w", err)
	}
	return nil
}
