package relay

import (
	"errors"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
)

type Config struct {
	Address       string `env:"RELAY_ADDRESS"`
	Domain        string `env:"RELAY_DOMAIN"` // the domain used for nip-42
	QueueCapacity int    `env:"RELAY_QUEUE_CAPACITY"`
	Processors    int    `env:"RELAY_PROCESSORS"`
	LogEvery      uint32 `env:"RELAY_LOG_EVERY"`
	SecretKey     string `env:"RELAY_SECRET_KEY"`
	PublicKey     string ``
}

// NewConfig returns a relay configuration structure with default paramenters.
func NewConfig() Config {
	return Config{
		Address:       "localhost:3334",
		QueueCapacity: 1000,
		Processors:    4,
		LogEvery:      1000,
	}
}

// Init initializes the config, deriving the public key from the secret key.
func (c *Config) Init() error {
	pk, err := nostr.GetPublicKey(c.SecretKey)
	if err != nil {
		return fmt.Errorf("Init: secret key is invalid: %w", err)
	}

	c.PublicKey = pk
	return nil
}

func (c Config) Validate() error {
	if c.QueueCapacity < 0 {
		return fmt.Errorf("queue capacity value must be positive: %d", c.QueueCapacity)
	}

	if c.Processors < 0 {
		return fmt.Errorf("processors value must be positive: %d", c.Processors)
	}

	if c.LogEvery == 0 {
		return fmt.Errorf("log every must be positive: %d", c.LogEvery)
	}

	if !nostr.IsValid32ByteHex(c.SecretKey) {
		return errors.New("secret key is invalid")
	}

	pk, err := nostr.GetPublicKey(c.SecretKey)
	if err != nil {
		return fmt.Errorf("secret key is invalid: %w", err)
	}

	if pk != c.PublicKey {
		return fmt.Errorf("public key and secret key don't match")
	}
	return nil
}

func (c Config) String() string {
	return fmt.Sprintf(
		"Relay:\n"+
			"\tAddress: %s\n"+
			"\tDomain: %s\n"+
			"\tQueue Capacity: %d\n"+
			"\tProcessors: %d\n"+
			"\tLogEvery: %d\n"+
			"\tSecretKey: %s\n"+
			"\tPublicKey: %s\n",
		c.Address,
		c.Domain,
		c.QueueCapacity,
		c.Processors,
		c.LogEvery,
		c.SecretKey[0:4]+"...REDACTED..."+c.SecretKey[len(c.SecretKey)-4:],
		c.PublicKey,
	)
}
