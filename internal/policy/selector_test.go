package policy

import (
	"errors"
	"testing"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
)

func TestSelectPrefersHealthyOverFasterDegraded(t *testing.T) {
	got, err := Select([]Candidate{
		{ID: "degraded", Status: engine.StatusDegraded, Latency: 1 * time.Millisecond},
		{ID: "healthy", Status: engine.StatusHealthy, Latency: 80 * time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "healthy" {
		t.Fatalf("expected healthy candidate, got %q", got.ID)
	}
}

func TestSelectLowestLatencyAndStableTieBreak(t *testing.T) {
	got, err := Select([]Candidate{
		{ID: "zeta", Status: engine.StatusHealthy, Latency: 20 * time.Millisecond},
		{ID: "beta", Status: engine.StatusHealthy, Latency: 20 * time.Millisecond},
		{ID: "alpha", Status: engine.StatusHealthy, Latency: 40 * time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "beta" {
		t.Fatalf("expected deterministic beta candidate, got %q", got.ID)
	}
}

func TestSelectUsesDegradedOnlyAsFallback(t *testing.T) {
	got, err := Select([]Candidate{
		{ID: "down", Status: engine.StatusFailed, Latency: time.Millisecond},
		{ID: "degraded", Status: engine.StatusDegraded, Latency: 40 * time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "degraded" {
		t.Fatalf("expected degraded fallback, got %q", got.ID)
	}
}

func TestSelectRejectsInvalidCandidates(t *testing.T) {
	_, err := Select([]Candidate{
		{ID: "", Status: engine.StatusHealthy, Latency: time.Millisecond},
		{ID: "negative", Status: engine.StatusHealthy, Latency: -time.Millisecond},
		{ID: "failed", Status: engine.StatusFailed, Latency: time.Millisecond},
	})
	if !errors.Is(err, ErrNoHealthyCandidate) {
		t.Fatalf("expected no candidate error, got %v", err)
	}
}
