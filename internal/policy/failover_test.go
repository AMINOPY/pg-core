package policy

import (
	"testing"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
)

func failoverCandidates(currentStatus engine.Status, currentLatency time.Duration) []ServerCandidate {
	return []ServerCandidate{
		{ID: "current", SubscriptionID: "sub-a", Status: currentStatus, Latency: currentLatency},
		{ID: "fast", SubscriptionID: "sub-b", Status: engine.StatusHealthy, Latency: 45 * time.Millisecond},
		{ID: "slower", SubscriptionID: "sub-a", Status: engine.StatusHealthy, Latency: 90 * time.Millisecond},
		{ID: "bad", SubscriptionID: "sub-c", Status: engine.StatusFailed, Latency: 1 * time.Millisecond},
		{ID: "unknown-latency", SubscriptionID: "sub-c", Status: engine.StatusHealthy, Latency: -1},
	}
}

func TestFailoverAfterConsecutiveFailuresAcrossSubscriptions(t *testing.T) {
	controller, err := NewFailoverController(FailoverConfig{
		FailureThreshold: 2, MaxLatency: 500 * time.Millisecond,
		Cooldown: time.Second, MinImprovement: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	candidates := failoverCandidates(engine.StatusFailed, -1)
	first, err := controller.Evaluate("current", candidates, now)
	if err != nil || first.Switch {
		t.Fatalf("first sample should not switch: decision=%+v err=%v", first, err)
	}
	second, err := controller.Evaluate("current", candidates, now.Add(100*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if !second.Switch || second.Target.ID != "fast" || second.Target.SubscriptionID != "sub-b" {
		t.Fatalf("expected best cross-subscription candidate: %+v", second)
	}
}

func TestFailoverOnHighLatencyAndCooldown(t *testing.T) {
	controller, err := NewFailoverController(FailoverConfig{
		FailureThreshold: 1, MaxLatency: 100 * time.Millisecond,
		Cooldown: time.Minute, MinImprovement: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	candidates := failoverCandidates(engine.StatusHealthy, 400*time.Millisecond)
	decision, err := controller.Evaluate("current", candidates, now)
	if err != nil || !decision.Switch {
		t.Fatalf("expected latency failover: decision=%+v err=%v", decision, err)
	}

	// Simulate a subsequent bad active server immediately after a switch.
	next := failoverCandidates(engine.StatusFailed, -1)
	next[1].ID = "current"
	next[1].SubscriptionID = "sub-b"
	next[1].Latency = 40 * time.Millisecond
	next[0].ID = "next"
	next[0].Status = engine.StatusFailed
	decision, err = controller.Evaluate("current", next, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if decision.Switch || decision.Reason != "cooldown" {
		t.Fatalf("expected cooldown to prevent flapping: %+v", decision)
	}
}

func TestFailoverRejectsNoAlternativesAndInvalidConfig(t *testing.T) {
	if _, err := NewFailoverController(FailoverConfig{}); err == nil {
		t.Fatal("expected invalid config error")
	}
	controller, err := NewFailoverController(FailoverConfig{
		FailureThreshold: 1, MaxLatency: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := controller.Evaluate("current", []ServerCandidate{
		{ID: "current", Status: engine.StatusFailed, Latency: -1},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if decision.Switch || decision.Reason != "no_healthy_alternative" {
		t.Fatalf("expected no-alternative result: %+v", decision)
	}
}

func TestFailoverDoesNotSwitchForSmallLatencyGain(t *testing.T) {
	controller, err := NewFailoverController(FailoverConfig{
		FailureThreshold: 1, MaxLatency: time.Second,
		MinImprovement: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	candidates := []ServerCandidate{
		{ID: "current", Status: engine.StatusHealthy, Latency: 300 * time.Millisecond},
		{ID: "other", SubscriptionID: "sub-z", Status: engine.StatusHealthy, Latency: 250 * time.Millisecond},
	}
	decision, err := controller.Evaluate("current", candidates, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if decision.Switch || decision.Reason != "current_healthy" {
		t.Fatalf("should not switch healthy connection before threshold: %+v", decision)
	}
}
