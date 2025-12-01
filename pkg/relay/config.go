package relay

import (
	"fmt"

	"github.com/nbd-wtf/go-nostr"
)

type Config struct {
	Address       string `env:"RELAY_ADDRESS"`
	Domain        string `env:"RELAY_DOMAIN"` // the domain used for nip-42
	QueueCapacity int    `env:"QUEUE_CAPACITY"`
	Processors    int    `env:"PROCESSORS"`
	SecretKey     string `env:"SECRET_KEY"`
	PublicKey     string ``
}

// NewConfig returns a relay configuration structure with default paramenters.
func NewConfig() Config {
	return Config{
		Address:       "localhost:3334",
		QueueCapacity: 1000,
		Processors:    4,
	}
}

func (c Config) Validate() error {
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

func (c Config) String() string {
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
