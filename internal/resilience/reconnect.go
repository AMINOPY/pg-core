// Package resilience provides bounded reconnect timing for unreliable networks.
// It intentionally leaves transport selection and platform routing to engine adapters.
package resilience

import (
	"errors"
	"math"
	"time"
)

var ErrInvalidPolicy = errors.New("invalid reconnect policy")

// Policy bounds reconnect delays and prevents aggressive retry loops on poor links.
// JitterFraction is in [0, 1]; callers can apply jitter with their own random source.
type Policy struct {
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	Multiplier    float64
	JitterFraction float64
}

// DefaultPolicy is conservative for metered and unstable mobile networks.
func DefaultPolicy() Policy {
	return Policy{
		InitialDelay:  500 * time.Millisecond,
		MaxDelay:      30 * time.Second,
		Multiplier:    2,
		JitterFraction: 0.2,
	}
}

func (p Policy) Validate() error {
	if p.InitialDelay <= 0 || p.MaxDelay < p.InitialDelay ||
		p.Multiplier < 1 || math.IsNaN(p.Multiplier) || math.IsInf(p.Multiplier, 0) ||
		p.JitterFraction < 0 || p.JitterFraction > 1 ||
		math.IsNaN(p.JitterFraction) || math.IsInf(p.JitterFraction, 0) {
		return ErrInvalidPolicy
	}
	return nil
}

// DelayFor returns the capped exponential delay for a zero-based consecutive
// failure number. The result is deterministic; apply bounded random jitter at
// the scheduling layer to avoid synchronized reconnect storms.
func (p Policy) DelayFor(consecutiveFailures int) (time.Duration, error) {
	if err := p.Validate(); err != nil {
		return 0, err
	}
	if consecutiveFailures < 0 {
		return 0, ErrInvalidPolicy
	}
	delay := float64(p.InitialDelay)
	for i := 0; i < consecutiveFailures; i++ {
		delay *= p.Multiplier
		if delay >= float64(p.MaxDelay) {
			return p.MaxDelay, nil
		}
	}
	if delay > float64(p.MaxDelay) {
		delay = float64(p.MaxDelay)
	}
	return time.Duration(delay), nil
}

// ApplyJitter clamps a supplied unit value to [0,1] and applies symmetric
// jitter. Pass a value from a concurrency-safe random source in production.
func (p Policy) ApplyJitter(delay time.Duration, unit float64) (time.Duration, error) {
	if err := p.Validate(); err != nil {
		return 0, err
	}
	if delay < 0 || math.IsNaN(unit) || math.IsInf(unit, 0) {
		return 0, ErrInvalidPolicy
	}
	if unit < 0 { unit = 0 }
	if unit > 1 { unit = 1 }
	factor := 1 - p.JitterFraction + (2*p.JitterFraction*unit)
	jittered := time.Duration(float64(delay) * factor)
	if jittered < 0 { jittered = 0 }
	if jittered > p.MaxDelay { jittered = p.MaxDelay }
	return jittered, nil
}
