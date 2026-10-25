package orchestrator

import (
	"context"
	"errors"
	"testing"
)

type startCountingAdapter struct {
	*fleetFakeAdapter
	starts int
}

func (a *startCountingAdapter) Start(ctx context.Context, cfg []byte) error {
	a.starts++
	return a.fleetFakeAdapter.Start(ctx, cfg)
}

func TestFleetRejectsRestartUntilFailedStopSucceeds(t *testing.T) {
	fleet := NewFleet()
	adapter := &startCountingAdapter{
		fleetFakeAdapter: &fleetFakeAdapter{
			fakeAdapter: healthyFake(),
			id:           "native",
		},
	}
	if err := fleet.Register(adapter); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := fleet.Start(ctx, "native", []byte("{}"), "tun:default"); err != nil {
		t.Fatal(err)
	}

	adapter.stopErr = errors.New("simulated route cleanup failure")
	if err := fleet.Stop(ctx, "native"); err == nil {
		t.Fatal("expected stop failure")
	}
	if err := fleet.Start(ctx, "native", []byte("{}"), "tun:default"); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("restart error = %v, want ErrAlreadyRunning", err)
	}
	if adapter.starts != 1 {
		t.Fatalf("adapter started %d times after failed cleanup, want 1", adapter.starts)
	}

	adapter.stopErr = nil
	if err := fleet.Stop(ctx, "native"); err != nil {
		t.Fatalf("retry stop: %v", err)
	}
	if err := fleet.Start(ctx, "native", []byte("{}"), "tun:default"); err != nil {
		t.Fatalf("start after successful cleanup: %v", err)
	}
	if adapter.starts != 2 {
		t.Fatalf("adapter start count = %d, want 2 after cleanup", adapter.starts)
	}
}
