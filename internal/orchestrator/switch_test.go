package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/AMINOPY/pg-core/internal/engine"
)

func TestSwitchWithRollbackRestoresCurrentAfterReplacementFailure(t *testing.T) {
	fleet := NewFleet()
	current := &fleetFakeAdapter{fakeAdapter: healthyFake(), id: "current"}
	replacement := &fleetFakeAdapter{
		fakeAdapter: &fakeAdapter{startErr: errors.New("replacement startup failed")},
		id:          "replacement",
	}
	for _, adapter := range []*fleetFakeAdapter{current, replacement} {
		if err := fleet.Register(adapter); err != nil {
			t.Fatal(err)
		}
	}
	if err := fleet.Start(context.Background(), "current", []byte(`{"version":1}`), "tun:default"); err != nil {
		t.Fatal(err)
	}

	selected, err := fleet.SwitchWithRollback(context.Background(),
		Candidate{EngineID: "current", Config: []byte(`{"version":1}`), Claims: []string{"tun:default"}},
		Candidate{EngineID: "replacement", Config: []byte(`{"version":2}`), Claims: []string{"tun:default"}},
	)
	if err == nil || !strings.Contains(err.Error(), "was restored") {
		t.Fatalf("error=%v, want replacement error and successful rollback", err)
	}
	if selected != "current" {
		t.Fatalf("selected=%q, want current after rollback", selected)
	}
	status, _, statusErr := fleet.Status("current")
	if statusErr != nil || status != engine.StatusHealthy {
		t.Fatalf("current status=%q err=%v, want healthy after rollback", status, statusErr)
	}
	if !replacement.stopped {
		t.Fatal("failed replacement should be cleaned up before rollback")
	}
}

func TestSwitchWithRollbackDoesNotRestoreWhileCleanupIsUncertain(t *testing.T) {
	fleet := NewFleet()
	current := &fleetFakeAdapter{fakeAdapter: healthyFake(), id: "current"}
	replacement := &fleetFakeAdapter{
		fakeAdapter: &fakeAdapter{
			startErr: errors.New("replacement startup failed"),
			stopErr:  errors.New("replacement cleanup failed"),
		},
		id: "replacement",
	}
	for _, adapter := range []*fleetFakeAdapter{current, replacement} {
		if err := fleet.Register(adapter); err != nil {
			t.Fatal(err)
		}
	}
	if err := fleet.Start(context.Background(), "current", nil, "tun:default"); err != nil {
		t.Fatal(err)
	}

	selected, err := fleet.SwitchWithRollback(context.Background(),
		Candidate{EngineID: "current", Claims: []string{"tun:default"}},
		Candidate{EngineID: "replacement", Claims: []string{"tun:default"}},
	)
	if err == nil || !strings.Contains(err.Error(), "original engine was not restarted") {
		t.Fatalf("error=%v, want cleanup uncertainty", err)
	}
	if selected != "" {
		t.Fatalf("selected=%q, want empty when resource ownership is uncertain", selected)
	}
	status, _, statusErr := fleet.Status("current")
	if statusErr != nil || status != engine.StatusStopped {
		t.Fatalf("current status=%q err=%v, want stopped", status, statusErr)
	}
	if err := fleet.Start(context.Background(), "current", nil, "tun:default"); !errors.Is(err, ErrResourceConflict) {
		t.Fatalf("current start error=%v, want retained resource conflict", err)
	}
}

func TestSwitchWithRollbackValidatesEngineIDs(t *testing.T) {
	fleet := NewFleet()
	if _, err := fleet.SwitchWithRollback(context.Background(), Candidate{}, Candidate{}); err == nil {
		t.Fatal("expected missing IDs to be rejected")
	}
	if _, err := fleet.SwitchWithRollback(context.Background(), Candidate{EngineID: "same"}, Candidate{EngineID: "same"}); err == nil {
		t.Fatal("expected identical IDs to be rejected")
	}
}
