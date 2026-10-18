package resilience

import (
	"context"
	"errors"
	"net"
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

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := operation(ctx)
		if err == nil {
			return nil
		}
		if !retryable(err) || attempt == maxAttempts-1 {
			return err
		}

		delay, err := policy.DelayFor(attempt)
		if err != nil {
			return err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

// IsTransientNetworkError is a conservative helper for common temporary network
// failures. Callers should classify engine-specific errors explicitly; unknown
// errors are not assumed retryable.
func IsTransientNetworkError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	return errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary())
}
