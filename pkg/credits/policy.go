package credits

import (
	"errors"
	"fmt"
	"time"
)

type RefillPolicy struct {
	Amount        int           `env:"CREDITS_REFILL_AMOUNT"`
	Interval      time.Duration `env:"CREDITS_REFILL_INTERVAL"`
	WalkThreshold int           `env:"CREDITS_REFILL_WALK_THRESHOLD"`
}

var NoRefill = RefillPolicy{Amount: 0}

func NewRefillPolicy() RefillPolicy {
	return RefillPolicy{
		Amount:        100,
		Interval:      24 * time.Hour,
		WalkThreshold: 5,
	}
}

func (p RefillPolicy) Validate() error {
	if p.Amount < 0 {
		return errors.New("amount cannot be negative")
	}

	if p.WalkThreshold < 0 {
		return errors.New("walk threshold cannot be negative")
	}
	return nil
}

func (p RefillPolicy) String() string {
	return fmt.Sprintf(
		"Credits Policy:\n"+
			"\tAmount: %d\n"+
			"\tInterval: %v\n"+
			"\tWalk Threshold: %d\n",
		p.Amount,
		p.Interval,
		p.WalkThreshold,
	)
}
