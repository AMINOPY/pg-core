package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/AMINOPY/pg-core/internal/engine"
)

type fleetFakeAdapter struct {
	*fakeAdapter
	id string
}

func (a *fleetFakeAdapter) ID() string { return a.id }

func TestFleetRunsIndependentEnginesAndIsolatesStop(t *testing.T) {
	fleet := NewFleet()
	first := &fleetFakeAdapter{fakeAdapter: healthyFake(), id: "native"}
	second := &fleetFakeAdapter{fakeAdapter: healthyFake(), id: "xray"}
	if err := fleet.Register(first); err != nil {
		t.Fatal(err)
	}
	if err := fleet.Register(second); err != nil {
		t.Fatal(err)
	}
	if err := fleet.Start(context.Background(), "native", []byte("{}"), "listener:native"); err != nil {
		t.Fatal(err)
	}
	if err := fleet.Start(context.Background(), "xray", []byte("{}"), "listener:xray"); err != nil {
		t.Fatal(err)
	}
	if err := fleet.Stop(context.Background(), "native"); err != nil {
		t.Fatal(err)
	}
	firstStatus, _, err := fleet.Status("native")
	if err != nil || firstStatus != engine.StatusStopped {
		t.Fatalf("native status=%q err=%v", firstStatus, err)
	}
	secondStatus, _, err := fleet.Status("xray")
	if err != nil || secondStatus != engine.StatusHealthy {
		t.Fatalf("xray should remain healthy after native stops: status=%q err=%v", secondStatus, err)
	}
	if second.stopped {
		t.Fatal("stopping native must not stop xray")
	}
}

func TestFleetRejectsConflictingExclusiveResourceClaims(t *testing.T) {
	fleet := NewFleet()
	first := &fleetFakeAdapter{fakeAdapter: healthyFake(), id: "engine-a"}
	second := &fleetFakeAdapter{fakeAdapter: healthyFake(), id: "engine-b"}
	if err := fleet.Register(first); err != nil {
		t.Fatal(err)
	}
	if err := fleet.Register(second); err != nil {
		t.Fatal(err)
	}
	if err := fleet.Start(context.Background(), "engine-a", []byte("{}"), "tun:default"); err != nil {
		t.Fatal(err)
	}
	if err := fleet.Start(context.Background(), "engine-b", []byte("{}"), "tun:default"); !errors.Is(err, ErrResourceConflict) {
		t.Fatalf("start conflict error=%v, want ErrResourceConflict", err)
	}
	if second.started {
		t.Fatal("engine with conflicting resource claim must not start")
	}
}

func TestFleetRejectsInvalidClaimsAndUnknownEngines(t *testing.T) {
	fleet := NewFleet()
	a := &fleetFakeAdapter{fakeAdapter: healthyFake(), id: "native"}
	if err := fleet.Register(a); err != nil {
		t.Fatal(err)
	}
	if err := fleet.Start(context.Background(), "native", []byte("{}"), "  "); !errors.Is(err, ErrInvalidResource) {
		t.Fatalf("invalid claim error=%v", err)
	}
	if err := fleet.Start(context.Background(), "missing", []byte("{}")); !errors.Is(err, ErrEngineNotRegistered) {
		t.Fatalf("unknown engine error=%v", err)
	}
	if a.started {
		t.Fatal("invalid request must not start an engine")
	}
}

func TestFleetRetainsClaimsAfterHealthFailureUntilStopSucceeds(t *testing.T) {
	fleet := NewFleet()
	first := &fleetFakeAdapter{fakeAdapter: healthyFake(), id: "engine-a"}
	second := &fleetFakeAdapter{fakeAdapter: healthyFake(), id: "engine-b"}
	for _, adapter := range []*fleetFakeAdapter{first, second} {
		if err := fleet.Register(adapter); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	if err := fleet.Start(ctx, "engine-a", []byte("{}"), "tun:default"); err != nil {
		t.Fatal(err)
	}

	// A failed data-plane health check triggers manager cleanup, but the fleet
	// must still require an explicit successful Stop before reassigning claims.
	first.health = engine.HealthReport{Status: engine.StatusFailed}
	report, err := fleet.RefreshHealth(ctx, "engine-a")
	if err != nil {
		t.Fatalf("RefreshHealth: %v", err)
	}
	if report.Status != engine.StatusFailed {
		t.Fatalf("health status=%q, want failed", report.Status)
	}
	if err := fleet.Start(ctx, "engine-b", []byte("{}"), "tun:default"); !errors.Is(err, ErrResourceConflict) {
		t.Fatalf("start after failed health error=%v, want ErrResourceConflict", err)
	}
	if second.started {
		t.Fatal("conflicting engine started while resource claim was retained")
	}

	if err := fleet.Stop(ctx, "engine-a"); err != nil {
		t.Fatalf("Stop failed engine: %v", err)
	}
	if err := fleet.Start(ctx, "engine-b", []byte("{}"), "tun:default"); err != nil {
		t.Fatalf("start after successful Stop: %v", err)
	}
}
