package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/AMINOPY/pg-core/internal/engine"
)

func TestStartFirstAvailableFallsBackInOrder(t *testing.T) {
	fleet := NewFleet()
	first := &fleetFakeAdapter{
		fakeAdapter: &fakeAdapter{startErr: errors.New("simulated startup failure")},
		id:          "primary",
	}
	second := &fleetFakeAdapter{fakeAdapter: healthyFake(), id: "backup"}
	for _, adapter := range []*fleetFakeAdapter{first, second} {
		if err := fleet.Register(adapter); err != nil {
			t.Fatal(err)
		}
	}

	selected, err := fleet.StartFirstAvailable(context.Background(), []Candidate{
		{EngineID: "primary", Config: []byte(`{}`), Claims: []string{"tun:default"}},
		{EngineID: "backup", Config: []byte(`{}`), Claims: []string{"tun:default"}},
	})
	if err != nil {
		t.Fatalf("StartFirstAvailable: %v", err)
	}
	if selected != "backup" {
		t.Fatalf("selected %q, want backup", selected)
	}
	if !first.stopped {
		t.Fatal("failed primary should be cleaned up before fallback")
	}
	if !second.started {
		t.Fatal("backup was not started")
	}
	status, _, err := fleet.Status("backup")
	if err != nil || status != engine.StatusHealthy {
		t.Fatalf("backup status=%q err=%v", status, err)
	}
}

func TestStartFirstAvailableStopsOnUncertainCleanup(t *testing.T) {
	fleet := NewFleet()
	first := &fleetFakeAdapter{
		fakeAdapter: &fakeAdapter{
			startErr: errors.New("startup failed"),
			stopErr:  errors.New("cleanup failed"),
		},
		id: "primary",
	}
	second := &fleetFakeAdapter{fakeAdapter: healthyFake(), id: "backup"}
	for _, adapter := range []*fleetFakeAdapter{first, second} {
		if err := fleet.Register(adapter); err != nil {
			t.Fatal(err)
		}
	}

	_, err := fleet.StartFirstAvailable(context.Background(), []Candidate{
		{EngineID: "primary", Claims: []string{"tun:default"}},
		{EngineID: "backup", Claims: []string{"tun:default"}},
	})
	if err == nil || !strings.Contains(err.Error(), "cleanup could not be confirmed") {
		t.Fatalf("error=%v, want cleanup uncertainty", err)
	}
	if second.started {
		t.Fatal("fallback must not start while primary cleanup is uncertain")
	}
	if err := fleet.Start(context.Background(), "backup", nil, "tun:default"); !errors.Is(err, ErrResourceConflict) {
		t.Fatalf("backup start error=%v, want retained resource conflict", err)
	}
}

func TestStartFirstAvailableValidatesCandidatesBeforeStarting(t *testing.T) {
	fleet := NewFleet()
	adapter := &fleetFakeAdapter{fakeAdapter: healthyFake(), id: "primary"}
	if err := fleet.Register(adapter); err != nil {
		t.Fatal(err)
	}

	for _, candidates := range [][]Candidate{
		nil,
		{{EngineID: ""}},
		{{EngineID: "primary"}, {EngineID: "primary"}},
		{{EngineID: " primary"}},
	} {
		if _, err := fleet.StartFirstAvailable(context.Background(), candidates); err == nil {
			t.Fatalf("candidates %+v: expected validation error", candidates)
		}
	}
	if adapter.started {
		t.Fatal("invalid candidate list must not start any engine")
	}
}

func TestStartFirstAvailableStopsOnResourceConflict(t *testing.T) {
	fleet := NewFleet()
	holder := &fleetFakeAdapter{fakeAdapter: healthyFake(), id: "holder"}
	backup := &fleetFakeAdapter{fakeAdapter: healthyFake(), id: "backup"}
	for _, adapter := range []*fleetFakeAdapter{holder, backup} {
		if err := fleet.Register(adapter); err != nil {
			t.Fatal(err)
		}
	}
	if err := fleet.Start(context.Background(), "holder", nil, "tun:default"); err != nil {
		t.Fatal(err)
	}
	_, err := fleet.StartFirstAvailable(context.Background(), []Candidate{
		{EngineID: "backup", Claims: []string{"tun:default"}},
	})
	if !errors.Is(err, ErrResourceConflict) {
		t.Fatalf("error=%v, want ErrResourceConflict", err)
	}
	if backup.started {
		t.Fatal("candidate must not start while resource is held by another engine")
	}
}


type cancelDuringStartAdapter struct {
	*fakeAdapter
	id         string
	cancel     context.CancelFunc
	stopCalls  int
	cleanupCtx error
}

func (a *cancelDuringStartAdapter) ID() string { return a.id }

func (a *cancelDuringStartAdapter) Start(context.Context, []byte) error {
	a.cancel()
	return errors.New("startup failed after cancellation")
}

func (a *cancelDuringStartAdapter) Stop(ctx context.Context) error {
	a.stopCalls++
	if a.stopCalls == 1 {
		return errors.New("initial cleanup failed")
	}
	a.cleanupCtx = ctx.Err()
	return nil
}

func TestStartFirstAvailableCleansUncertainCandidateAfterCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fleet := NewFleet()
	first := &cancelDuringStartAdapter{
		fakeAdapter: healthyFake(),
		id:          "primary",
		cancel:      cancel,
	}
	backup := &fleetFakeAdapter{fakeAdapter: healthyFake(), id: "backup"}
	for _, adapter := range []interface {
		ID() string
		Capabilities() engine.Capabilities
		ValidateConfig(context.Context, []byte) error
		Start(context.Context, []byte) error
		Health(context.Context) (engine.HealthReport, error)
		Stop(context.Context) error
	}{first, backup} {
		if err := fleet.Register(adapter); err != nil {
			t.Fatal(err)
		}
	}

	_, err := fleet.StartFirstAvailable(ctx, []Candidate{
		{EngineID: "primary", Claims: []string{"tun:default"}},
		{EngineID: "backup", Claims: []string{"tun:default"}},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v, want caller cancellation after cleanup", err)
	}
	if first.stopCalls != 2 {
		t.Fatalf("stop calls=%d, want initial cleanup plus independent recovery cleanup", first.stopCalls)
	}
	if first.cleanupCtx != nil {
		t.Fatalf("recovery cleanup inherited cancelled context: %v", first.cleanupCtx)
	}
	if backup.started {
		t.Fatal("backup must not start after caller cancellation")
	}
}
