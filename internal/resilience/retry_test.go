package resilience

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunRetriesThenSucceeds(t *testing.T) {
	p := Policy{InitialDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond, Multiplier: 2}
	attempts := 0
	err := Run(context.Background(), p, 4, func(error) bool { return true }, func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("got %d attempts, want 3", attempts)
	}
}

func TestRunDoesNotRetryPermanentError(t *testing.T) {
	want := errors.New("invalid configuration")
	attempts := 0
	err := Run(context.Background(), DefaultPolicy(), 5, func(error) bool { return false }, func(context.Context) error {
		attempts++
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
	if attempts != 1 {
		t.Fatalf("got %d attempts, want 1", attempts)
	}
}

func TestRunHonorsCancellationDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var attempts atomic.Int32
	done := make(chan error, 1)
	p := Policy{InitialDelay: time.Second, MaxDelay: time.Second, Multiplier: 1}
	go func() {
		done <- Run(ctx, p, 3, func(error) bool { return true }, func(context.Context) error {
			attempts.Add(1)
			return errors.New("temporary")
		})
	}()
	deadline := time.After(time.Second)
	for attempts.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("operation was not called")
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancellation")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("got %d attempts, want 1", got)
	}
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	if err := Run(context.Background(), DefaultPolicy(), 0, func(error) bool { return true }, func(context.Context) error { return nil }); !errors.Is(err, ErrInvalidPolicy) {
		t.Fatalf("got %v, want ErrInvalidPolicy", err)
	}
}
