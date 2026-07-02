package openranking

import (
	"errors"
	"time"

	"github.com/vertex-lab/relay/pkg/credits"
	"github.com/vertex-lab/relay/pkg/ranking"
)

type Config struct {
	Ranking ranking.Config
	Refill  credits.RefillPolicy

	Address           string        `env:"ADDRESS"`
	ReadHeaderTimeout time.Duration `env:"READ_HEADER_TIMEOUT"`
	IdleTimeout       time.Duration `env:"IDLE_TIMEOUT"`
	RequestTimeout    time.Duration `env:"REQUEST_TIMEOUT"`
	ShutdownTimeout   time.Duration `env:"SHUTDOWN_TIMEOUT"`
}

func NewConfig() Config {
	return Config{
		Ranking:           ranking.NewConfig(),
		Refill:            credits.NewRefillPolicy(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		ShutdownTimeout:   5 * time.Second,
	}
}

func (c Config) Validate() error {
	if err := c.Ranking.Validate(); err != nil {
		return err
	}
	if err := c.Refill.Validate(); err != nil {
		return err
	}
	if c.ReadHeaderTimeout < time.Second {
		return errors.New("read header timeout must be at least 1 second to function reliably")
	}
	if c.IdleTimeout < 10*time.Second {
		return errors.New("idle timeout must be at least 10 seconds to function reliably")
	}
	if c.RequestTimeout < time.Second {
		return errors.New("request timeout must be at least 1 second to function reliably")
	}
	if c.ShutdownTimeout < time.Second {
		return errors.New("shutdown timeout must be at least 1 second to function reliably")
	}
	return nil
}

func (c Config) String() string {
	// TODO: improve using string builder.
	return c.Ranking.String() + "\n" + c.Refill.String()
}
