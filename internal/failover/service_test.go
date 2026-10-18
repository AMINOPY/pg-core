package failover

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
	"github.com/AMINOPY/pg-core/internal/health"
	"github.com/AMINOPY/pg-core/internal/policy"
)

type fakeSampler struct {
	results []health.ProbeResult
	err     error
}

func (f fakeSampler) Probe(context.Context, []health.Target) ([]health.ProbeResult, error) {
	return f.results, f.err
}

type fakeSwitcher struct {
	switchErr   error
	verifyErr   error
	rollbackErr error
	switched    []string
	rolledBack  bool
}

func (f *fakeSwitcher) Switch(_ context.Context, from, to string) error {
	f.switched = append(f.switched, from+"->"+to)
	return f.switchErr
}
func (f *fakeSwitcher) Verify(context.Context, string) error { return f.verifyErr }
func (f *fakeSwitcher) Rollback(context.Context, string, string) error {
	f.rolledBack = true
	return f.rollbackErr
}

func newService(t *testing.T, sampler Sampler, switcher Switcher) *Service {
	t.Helper()
	controller, err := policy.NewFailoverController(policy.FailoverConfig{
		FailureThreshold: 1, MaxLatency: 500 * time.Millisecond,
		Cooldown: time.Minute, MinImprovement: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(controller, sampler, switcher)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestRunOnceSwitchesAndVerifies(t *testing.T) {
	sampler := fakeSampler{results: []health.ProbeResult{
		{Candidate: policy.ServerCandidate{ID: "current", SubscriptionID: "sub-a", Status: engine.StatusFailed, Latency: -1}},
		{Candidate: policy.ServerCandidate{ID: "next", SubscriptionID: "sub-b", Status: engine.StatusHealthy, Latency: 20 * time.Millisecond}},
	}}
	switcher := &fakeSwitcher{}
	service := newService(t, sampler, switcher)
	outcome, err := service.RunOnce(context.Background(), "current", []health.Target{{ID: "current", URL: "https://health.example"}})
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Switched || outcome.Decision.Target.ID != "next" {
		t.Fatalf("expected successful switch: %+v", outcome)
	}
	if len(switcher.switched) != 1 || switcher.switched[0] != "current->next" {
		t.Fatalf("unexpected switch calls: %+v", switcher.switched)
	}
}

func TestRunOnceRollsBackWhenVerificationFails(t *testing.T) {
	sampler := fakeSampler{results: []health.ProbeResult{
		{Candidate: policy.ServerCandidate{ID: "current", Status: engine.StatusFailed, Latency: -1}},
		{Candidate: policy.ServerCandidate{ID: "next", Status: engine.StatusHealthy, Latency: 20 * time.Millisecond}},
	}}
	switcher := &fakeSwitcher{verifyErr: errors.New("DNS verification failed")}
	service := newService(t, sampler, switcher)
	_, err := service.RunOnce(context.Background(), "current", []health.Target{{ID: "current", URL: "https://health.example"}})
	if err == nil || !switcher.rolledBack {
		t.Fatalf("expected verification error and rollback, err=%v rolledBack=%v", err, switcher.rolledBack)
	}
}

func TestRunOnceCancelsFailedSwitch(t *testing.T) {
	sampler := fakeSampler{results: []health.ProbeResult{
		{Candidate: policy.ServerCandidate{ID: "current", Status: engine.StatusFailed, Latency: -1}},
		{Candidate: policy.ServerCandidate{ID: "next", Status: engine.StatusHealthy, Latency: 20 * time.Millisecond}},
	}}
	switcher := &fakeSwitcher{switchErr: errors.New("start failed")}
	service := newService(t, sampler, switcher)
	_, err := service.RunOnce(context.Background(), "current", []health.Target{{ID: "current", URL: "https://health.example"}})
	if err == nil {
		t.Fatal("expected switch error")
	}
}
