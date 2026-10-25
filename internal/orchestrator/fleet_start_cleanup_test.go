package orchestrator

import (
	"context"
	"errors"
	"testing"
)

func TestFleetRetainsClaimsWhenStartupCleanupFails(t *testing.T) {
	fleet := NewFleet()
	first := &fleetFakeAdapter{fakeAdapter: healthyFake(), id: "engine-a"}
	second := &fleetFakeAdapter{fakeAdapter: healthyFake(), id: "engine-b"}
	for _, adapter := range []*fleetFakeAdapter{first, second} {
		if err := fleet.Register(adapter); err != nil {
			t.Fatal(err)
		}
	}

	first.startErr = errors.New("simulated partial startup failure")
	first.stopErr = errors.New("simulated cleanup failure")
	ctx := context.Background()
	err := fleet.Start(ctx, "engine-a", []byte("{}"), "tun:default")
	if err == nil || !errors.Is(err, ErrResourceStateUncertain) {
		t.Fatalf("start error = %v, want ErrResourceStateUncertain", err)
	}
	if err := fleet.Start(ctx, "engine-a", []byte("{}"), "tun:default"); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("same-engine restart error = %v, want ErrAlreadyRunning", err)
	}
	if err := fleet.Start(ctx, "engine-b", []byte("{}"), "tun:default"); !errors.Is(err, ErrResourceConflict) {
		t.Fatalf("conflicting engine start error = %v, want ErrResourceConflict", err)
	}
	if second.started {
		t.Fatal("conflicting engine started while startup cleanup was incomplete")
	}

	first.stopErr = nil
	if err := fleet.Stop(ctx, "engine-a"); err != nil {
		t.Fatalf("retry cleanup: %v", err)
	}
	first.startErr = nil
	if err := fleet.Start(ctx, "engine-b", []byte("{}"), "tun:default"); err != nil {
		t.Fatalf("start after cleanup succeeds: %v", err)
	}
}
