package resilience

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Retryable reports whether an operation error is transient and may be retried.
// Permanent errors (for example invalid configuration or authentication failures)
// should return false.
type Retryable func(error) bool

// Operation is one attempt to establish or restore a connection.
type Operation func(context.Context) error

// Run executes an operation with bounded exponential backoff. maxAttempts counts
// the initial attempt; a value of 1 disables retries. It stops immediately on
// success, a permanent error, or context cancellation.
func Run(ctx context.Context, policy Policy, maxAttempts int, retryable Retryable, operation Operation) error {
	if ctx == nil || retryable == nil || operation == nil || maxAttempts < 1 {
		return ErrInvalidPolicy
	}
	if err := policy.Validate(); err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = operation(ctx)
		if lastErr == nil {
			return nil
		}
		if !retryable(lastErr) || attempt == maxAttempts-1 {
			return lastErr
		}

		delay, err := policy.DelayFor(attempt)
		if err != nil {
			return err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	if lastErr == nil {
		return nil
	}
	return fmt.Errorf("retry attempts exhausted: %w", lastErr)
}

// IsTransientNetworkError is a conservative helper for common temporary network
// failures. Callers should classify engine-specific errors explicitly; unknown
// errors are not assumed retryable.
func IsTransientNetworkError(err error) bool {
	if err == nil {
		return false
	}
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var temporary interface{ Temporary() bool }
	if errors.As(err, &temporary) && temporary.Temporary() {
		return true
	}
	return false
}
