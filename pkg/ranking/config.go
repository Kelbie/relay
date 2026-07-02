package ranking

import (
	"fmt"
)

type Config struct {
	RedisAddress string `env:"REDIS_ADDRESS"`
	SqlitePath   string `env:"SQLITE_PATH"`
}

func NewConfig() Config {
	return Config{
		RedisAddress: "localhost:6379",
		SqlitePath:   "relay.sqlite",
	}
}

func (c Config) String() string {
	return fmt.Sprintf("Service:\n"+
		"\tRedis Address: %s\n"+
		"\tSqlite Path: %s\n",
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
	return nil
}
