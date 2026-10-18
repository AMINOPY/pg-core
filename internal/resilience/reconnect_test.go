package resilience

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestDelayForCapsExponentialBackoff(t *testing.T) {
	p := DefaultPolicy()
	cases := []struct {
		failures int
		want     time.Duration
	}{
		{0, 500 * time.Millisecond},
		{1, time.Second},
		{2, 2 * time.Second},
		{6, 30 * time.Second},
		{20, 30 * time.Second},
	}
	for _, tc := range cases {
		got, err := p.DelayFor(tc.failures)
		if err != nil {
			t.Fatalf("DelayFor(%d): %v", tc.failures, err)
		}
		if got != tc.want {
			t.Errorf("DelayFor(%d) = %s, want %s", tc.failures, got, tc.want)
		}
	}
}

func TestApplyJitterBounds(t *testing.T) {
	p := DefaultPolicy()
	for _, tc := range []struct {
		unit float64
		want time.Duration
	}{
		{0, 400 * time.Millisecond},
		{0.5, 500 * time.Millisecond},
		{1, 600 * time.Millisecond},
		{2, 600 * time.Millisecond},
	} {
		got, err := p.ApplyJitter(500*time.Millisecond, tc.unit)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("ApplyJitter(%v) = %s, want %s", tc.unit, got, tc.want)
		}
	}
}

func TestRejectsInvalidPolicyAndFailureCount(t *testing.T) {
	p := DefaultPolicy()
	p.Multiplier = math.NaN()
	if _, err := p.DelayFor(0); !errors.Is(err, ErrInvalidPolicy) {
		t.Fatalf("expected ErrInvalidPolicy, got %v", err)
	}
	if _, err := DefaultPolicy().DelayFor(-1); !errors.Is(err, ErrInvalidPolicy) {
		t.Fatalf("expected ErrInvalidPolicy, got %v", err)
	}
}
