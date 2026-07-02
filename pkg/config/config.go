// The package config defined and loads the configuration parameters from the .env file.
package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	_ "github.com/joho/godotenv/autoload"

	"github.com/vertex-lab/relay/pkg/api"
	"github.com/vertex-lab/relay/pkg/core"
	openranking "github.com/vertex-lab/relay/pkg/open-ranking"
	"github.com/vertex-lab/relay/pkg/ranking"
	"github.com/vertex-lab/relay/pkg/rate"
	"github.com/vertex-lab/relay/pkg/relay"
)

type Config struct {
	Core        core.Config
	Service     ranking.Config
	OpenRanking openranking.Config
	Limiter     rate.Config
	Relay       relay.Config
	API         api.Config
}

// New returns a config with default paramenters.
func New() Config {
	return Config{
		Core:        core.NewConfig(),
		Service:     ranking.NewConfig(),
		OpenRanking: openranking.NewConfig(),
		Limiter:     rate.NewConfig(),
		Relay:       relay.NewConfig(),
		API:         api.NewConfig(),
	}
}

func (c Config) Validate() error {
	if err := c.Core.Validate(); err != nil {
		return fmt.Errorf("Core: %w", err)
	}
	if err := c.Service.Validate(); err != nil {
		return fmt.Errorf("Service: %w", err)
	}
	if err := c.OpenRanking.Validate(); err != nil {
		return fmt.Errorf("OpenRanking: %w", err)
	}
	if err := c.Limiter.Validate(); err != nil {
		return fmt.Errorf("Limiter: %w", err)
	}
	if err := c.Relay.Validate(); err != nil {
		return fmt.Errorf("Relay: %w", err)
	}
	if err := c.API.Validate(); err != nil {
		return fmt.Errorf("API: %w", err)
	}
	return nil
}

func (c Config) Print() {
	fmt.Println(c.Core)
	fmt.Println(c.Service)
	fmt.Println(c.OpenRanking)
	fmt.Println(c.Limiter)
	fmt.Println(c.Relay)
	fmt.Println(c.API)
}

// Load creates a new [Config] with default parameters, that get overwritten by
// env variables when specified.
func Load() (Config, error) {
	config := New()
	err := env.Parse(&config)
	if err != nil {
		return Config{}, fmt.Errorf("config.Load: %w", err)
	}

	config.Relay.Init()

	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("config.Load: %w", err)
	}
	return config, nil
}
