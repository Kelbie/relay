package rate

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

type LimiterConfig struct {
	Trusted   trustedConfig
	Default   defaultConfig
	Untrusted untrustedConfig
}

type trustedConfig struct {
	List     []string     `envconfig:"RATE_TRUSTED"`
	Refiller FlatRefiller `envconfig:"RATE_TRUSTED"`
}

type untrustedConfig struct {
	List     []string     `envconfig:"RATE_UNTRUSTED"`
	Refiller FlatRefiller `envconfig:"RATE_UNTRUSTED"`
}

type defaultConfig struct {
	Refiller FlatRefiller `envconfig:"RATE_DEFAULT"`
}

func NewConfig() LimiterConfig {
	return LimiterConfig{
		Trusted: trustedConfig{
			Refiller: FlatRefiller{
				InitialTokens:     10_000,
				MaxTokens:         10_000,
				TokensPerInterval: 2000,
				Interval:          time.Minute,
			},
		},
		Default: defaultConfig{
			Refiller: FlatRefiller{
				InitialTokens:     300,
				MaxTokens:         300,
				TokensPerInterval: 100,
				Interval:          time.Minute,
			},
		},
		Untrusted: untrustedConfig{},
	}
}

func NewDynamicRefiller(config LimiterConfig) DynamicRefiller {
	return DynamicRefiller{
		Resolve: func(entity string) Refiller {
			if slices.Contains(config.Untrusted.List, entity) {
				return config.Untrusted.Refiller
			}
			if slices.Contains(config.Trusted.List, entity) {
				return config.Trusted.Refiller
			}
			return config.Default.Refiller
		},
	}
}

func (c LimiterConfig) String() string {
	sb := strings.Builder{}
	sb.WriteString("Limiter Config:\n")
	sb.WriteString("\tTrusted:\n")
	sb.WriteString(fmt.Sprintf("\t\tList: %v\n", c.Trusted.List))
	sb.WriteString(fmt.Sprintf("\t\t%s", strings.ReplaceAll(c.Trusted.Refiller.String(), "\n", "\n\t\t")))
	sb.WriteString("\n")

	sb.WriteString("\tUntrusted:\n")
	sb.WriteString(fmt.Sprintf("\t\tList: %v\n", c.Untrusted.List))
	sb.WriteString(fmt.Sprintf("\t\t%s", strings.ReplaceAll(c.Untrusted.Refiller.String(), "\n", "\n\t\t")))
	sb.WriteString("\n")

	sb.WriteString("\tDefault:\n")
	sb.WriteString(fmt.Sprintf("\t\t%s", strings.ReplaceAll(c.Default.Refiller.String(), "\n", "\n\t\t")))
	sb.WriteString("\n")
	return sb.String()
}

func (r FlatRefiller) String() string {
	return fmt.Sprintf(
		"Initial Tokens: %g\n"+
			"Max Tokens: %g\n"+
			"Tokens Per Interval: %g\n"+
			"Interval: %v",
		r.InitialTokens,
		r.MaxTokens,
		r.TokensPerInterval,
		r.Interval,
	)
}

func (c LimiterConfig) Validate() error {
	if err := c.Trusted.Refiller.ValidateActive(); err != nil {
		return fmt.Errorf("trusted: %w", err)
	}
	if err := c.Default.Refiller.ValidateActive(); err != nil {
		return fmt.Errorf("default: %w", err)
	}
	if err := c.Untrusted.Refiller.Validate(); err != nil {
		return fmt.Errorf("untrusted: %w", err)
	}
	return nil
}

// Validate returns an error if the refiller is badly configured, else nil.
func (r FlatRefiller) Validate() error {
	if r.InitialTokens < 0 {
		return errors.New("initial tokens cannot be negative")
	}
	if r.MaxTokens < 0 {
		return errors.New("max tokens cannot be negative")
	}
	if r.InitialTokens > r.MaxTokens {
		return errors.New("initial tokens cannot be more than max tokens")
	}
	if r.TokensPerInterval < 0 {
		return errors.New("tokens per interval cannot be negative")
	}
	if r.Interval < 0 {
		return errors.New("interval cannot be negative")
	}
	return nil
}

// ValidateActive returns an error if the refiller is badly configured,
// or if the refiller is never going to refill.
func (r FlatRefiller) ValidateActive() error {
	if r.InitialTokens < 0 {
		return errors.New("initial tokens cannot be negative")
	}
	if r.MaxTokens <= 0 {
		return errors.New("max tokens must be positive for a non-empty refill")
	}
	if r.InitialTokens > r.MaxTokens {
		return errors.New("initial tokens cannot be more than max tokens")
	}
	if r.TokensPerInterval <= 0 {
		return errors.New("tokens per interval must be positive for a non-empty refill")
	}
	if r.Interval <= 0 {
		return errors.New("interval must be positive for a non-empty refill")
	}
	return nil
}
