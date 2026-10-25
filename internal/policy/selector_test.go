package policy

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
)

func TestSelectPrefersHealthyOverFasterDegraded(t *testing.T) {
	got, err := Select([]Candidate{
		{ID: "degraded", Status: engine.StatusDegraded, Latency: time.Millisecond},
		{ID: "healthy", Status: engine.StatusHealthy, Latency: 80 * time.Millisecond},
	})
	if err != nil { t.Fatal(err) }
	if got.ID != "healthy" { t.Fatalf("expected healthy candidate, got %q", got.ID) }
}

func TestSelectLowestLatencyAndStableTieBreak(t *testing.T) {
	got, err := Select([]Candidate{
		{ID: "zeta", Status: engine.StatusHealthy, Latency: 20 * time.Millisecond},
		{ID: "beta", Status: engine.StatusHealthy, Latency: 20 * time.Millisecond},
		{ID: "alpha", Status: engine.StatusHealthy, Latency: 40 * time.Millisecond},
	})
	if err != nil { t.Fatal(err) }
	if got.ID != "beta" { t.Fatalf("expected deterministic beta candidate, got %q", got.ID) }
}

func TestSelectUsesDegradedOnlyAsFallback(t *testing.T) {
	got, err := Select([]Candidate{
		{ID: "down", Status: engine.StatusFailed, Latency: time.Millisecond},
		{ID: "degraded", Status: engine.StatusDegraded, Latency: 40 * time.Millisecond},
	})
	if err != nil { t.Fatal(err) }
	if got.ID != "degraded" { t.Fatalf("expected degraded fallback, got %q", got.ID) }
}

func TestSelectUsesVerifiedQualityMetrics(t *testing.T) {
	got, err := Select([]Candidate{
		{ID: "fast-but-lossy", Status: engine.StatusHealthy, Latency: 10 * time.Millisecond,
			MetricsValid: true, SuccessRate: .80, PacketLoss: .20, Jitter: time.Millisecond, ThroughputMbps: 100},
		{ID: "stable", Status: engine.StatusHealthy, Latency: 80 * time.Millisecond,
			MetricsValid: true, SuccessRate: .999, PacketLoss: .001, Jitter: 5 * time.Millisecond, ThroughputMbps: 30},
	})
	if err != nil { t.Fatal(err) }
	if got.ID != "stable" { t.Fatalf("expected stable candidate, got %q", got.ID) }
}

func TestSelectPrefersMeasuredCandidateOverUnmeasuredAtSameHealth(t *testing.T) {
	got, err := Select([]Candidate{
		{ID: "unknown", Status: engine.StatusHealthy, Latency: time.Millisecond},
		{ID: "measured", Status: engine.StatusHealthy, Latency: 100 * time.Millisecond,
			MetricsValid: true, SuccessRate: .99, PacketLoss: .01, Jitter: time.Millisecond, ThroughputMbps: 10},
	})
	if err != nil { t.Fatal(err) }
	if got.ID != "measured" { t.Fatalf("expected measured candidate, got %q", got.ID) }
}

func TestSelectRejectsInvalidCandidates(t *testing.T) {
	_, err := Select([]Candidate{
		{ID: "", Status: engine.StatusHealthy, Latency: time.Millisecond},
		{ID: "negative", Status: engine.StatusHealthy, Latency: -time.Millisecond},
		{ID: "bad-loss", Status: engine.StatusHealthy, Latency: time.Millisecond, MetricsValid: true, PacketLoss: 1.1},
		{ID: "nan-throughput", Status: engine.StatusHealthy, Latency: time.Millisecond, MetricsValid: true, ThroughputMbps: math.NaN()},
		{ID: "failed", Status: engine.StatusFailed, Latency: time.Millisecond},
	})
	if !errors.Is(err, ErrNoHealthyCandidate) { t.Fatalf("expected no candidate error, got %v", err) }
}

func TestSelectQualityTieBreaksByLatencyThenJitterAndThroughput(t *testing.T) {
	got, err := Select([]Candidate{
		{ID: "slow", Status: engine.StatusHealthy, Latency: 50 * time.Millisecond, MetricsValid: true, SuccessRate: 1, PacketLoss: 0, Jitter: time.Millisecond, ThroughputMbps: 20},
		{ID: "fast", Status: engine.StatusHealthy, Latency: 20 * time.Millisecond, MetricsValid: true, SuccessRate: 1, PacketLoss: 0, Jitter: 8 * time.Millisecond, ThroughputMbps: 5},
	})
	if err != nil { t.Fatal(err) }
	if got.ID != "fast" { t.Fatalf("expected latency tie-break, got %q", got.ID) }
}
