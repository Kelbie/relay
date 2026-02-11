package rate

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/pippellia-btc/rate"
)

// Config is the configuration for the rate limiter.
// It contains 3 [rate.Refiller]s:
// - Trusted: for trusted clients in the TrustedList
// - Default: for default clients
// - Untrusted: for untrusted clients in the UntrustedList
type Config struct {
	TrustedList []string `env:"RATE_TRUSTED_LIST"`
	Trusted     Refiller `envPrefix:"RATE_TRUSTED_"`

	Unknown Refiller `envPrefix:"RATE_UNKNOWN_"`

	UntrustedList []string `env:"RATE_UNTRUSTED_LIST"`
	Untrusted     Refiller `envPrefix:"RATE_UNTRUSTED_"`
}

func NewConfig() Config {
	return Config{
		Trusted: Refiller{
			InitialTokens: 10_000, MaxTokens: 10_000, TokensPerInterval: 10_000, Interval: time.Minute,
		},
		Unknown: Refiller{
			InitialTokens: 300, MaxTokens: 300, TokensPerInterval: 100, Interval: time.Minute,
		},
	}
}

func (c Config) NewBucket(entity string) *rate.Bucket {
	switch {
	case slices.Contains(c.TrustedList, entity):
		return c.Trusted.NewBucket(entity)

	case slices.Contains(c.UntrustedList, entity):
		return c.Untrusted.NewBucket(entity)

	default:
		return c.Unknown.NewBucket(entity)
	}
}

func (c Config) Refill(entity string, b *rate.Bucket) {
	switch {
	case slices.Contains(c.TrustedList, entity):
		c.Trusted.Refill(entity, b)

	case slices.Contains(c.UntrustedList, entity):
		c.Untrusted.Refill(entity, b)

	default:
		c.Unknown.Refill(entity, b)
	}
}

// Refiller is a [rate.Refiller]. An empty Refiller will block all requests.
type Refiller struct {
	InitialTokens     float64       `env:"INITIAL_TOKENS"`
	MaxTokens         float64       `env:"MAX_TOKENS"`
	TokensPerInterval float64       `env:"TOKENS_PER_INTERVAL"`
	Interval          time.Duration `env:"INTERVAL"`
}

func (r Refiller) NewBucket(_ string) *rate.Bucket {
	return &rate.Bucket{
		Tokens:     r.InitialTokens,
		LastRefill: time.Now(),
	}
}

func (r Refiller) Refill(_ string, b *rate.Bucket) {
	if r.Interval <= 0 {
		return
	}
	refills := time.Since(b.LastRefill) / r.Interval
	if refills == 0 {
		return
	}

	b.Tokens = min(r.MaxTokens, b.Tokens+float64(refills)*r.TokensPerInterval)
	b.LastRefill = b.LastRefill.Add(refills * r.Interval)
}

// Validate returns an error if the refiller is badly configured, else nil.
func (r Refiller) Validate() error {
	if r.InitialTokens < 0 {
		return errors.New("initial tokens cannot be negative")
	}
	if r.MaxTokens < 0 {
		return errors.New("max tokens cannot be negative")
	}
	if r.TokensPerInterval < 0 {
		return errors.New("tokens per interval cannot be negative")
	}
	if r.Interval < 0 {
		return errors.New("interval cannot be negative")
	}

	if r.InitialTokens > r.MaxTokens {
		return errors.New("initial tokens cannot be more than max tokens")
	}
	if r.TokensPerInterval > r.MaxTokens {
		return errors.New("tokens per interval cannot be more than max tokens")
	}
	return nil
}

func (c Config) String() string {
	sb := strings.Builder{}
	sb.WriteString("Limiter:\n")
	sb.WriteString("\tTrusted:\n")
	sb.WriteString(fmt.Sprintf("\t\tList: %v\n", c.TrustedList))
	sb.WriteString(fmt.Sprintf("\t\t%s", strings.ReplaceAll(c.Trusted.String(), "\n", "\n\t\t")))
	sb.WriteString("\n")

	sb.WriteString("\tUntrusted:\n")
	sb.WriteString(fmt.Sprintf("\t\tList: %v\n", c.UntrustedList))
	sb.WriteString(fmt.Sprintf("\t\t%s", strings.ReplaceAll(c.Untrusted.String(), "\n", "\n\t\t")))
	sb.WriteString("\n")

	sb.WriteString("\tUnknown:\n")
	sb.WriteString(fmt.Sprintf("\t\t%s", strings.ReplaceAll(c.Unknown.String(), "\n", "\n\t\t")))
	sb.WriteString("\n")
	return sb.String()
}

func (r Refiller) String() string {
	return fmt.Sprintf("Initial Tokens: %g\n"+
		"Max Tokens: %g\n"+
		"Tokens Per Interval: %g\n"+
		"Interval: %v",
		r.InitialTokens,
		r.MaxTokens,
		r.TokensPerInterval,
		r.Interval,
	)
}

func (c Config) Validate() error {
	if err := c.Trusted.Validate(); err != nil {
		return fmt.Errorf("trusted: %w", err)
	}
	if err := c.Unknown.Validate(); err != nil {
		return fmt.Errorf("unknown: %w", err)
	}
	if err := c.Untrusted.Validate(); err != nil {
		return fmt.Errorf("untrusted: %w", err)
	}
	return nil
}
