package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
)

type fakeAdapter struct {
	startErr    error
	validateErr error
	health      engine.HealthReport
	healthErr   error
	stopErr     error
	started     bool
	stopped     bool
}

func (f *fakeAdapter) ID() string { return "fake" }

func (f *fakeAdapter) Capabilities() engine.Capabilities {
	return engine.Capabilities{HealthChecks: true}
}

func (f *fakeAdapter) ValidateConfig(context.Context, []byte) error {
	return f.validateErr
}

func (f *fakeAdapter) Start(context.Context, []byte) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.started = true
	return nil
}

func (f *fakeAdapter) Health(context.Context) (engine.HealthReport, error) {
	return f.health, f.healthErr
}

func (f *fakeAdapter) Stop(context.Context) error {
	if f.stopErr != nil {
		return f.stopErr
	}
	f.stopped = true
	return nil
}

func healthyFake() *fakeAdapter {
	return &fakeAdapter{health: engine.HealthReport{
		Status: engine.StatusHealthy, CheckedAt: time.Now().UTC(),
	}}
}

func TestStartAndStop(t *testing.T) {
	a := healthyFake()
	m, err := New(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(context.Background(), []byte("{}")); err != nil {
		t.Fatal(err)
	}
	status, report := m.Status()
	if status != engine.StatusHealthy || report.EngineID != "fake" {
		t.Fatalf("unexpected status/report: %q %+v", status, report)
	}
	if err := m.Start(context.Background(), nil); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("expected already-running error, got %v", err)
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, _ = m.Status()
	if status != engine.StatusStopped {
		t.Fatalf("expected stopped, got %q", status)
	}
}

func TestInvalidConfigDoesNotStartEngine(t *testing.T) {
	a := healthyFake()
	a.validateErr = errors.New("bad config")
	m, _ := New(a)
	if err := m.Start(context.Background(), nil); err == nil {
		t.Fatal("expected validation error")
	}
	if a.started {
		t.Fatal("engine must not start with invalid config")
	}
	status, _ := m.Status()
	if status != engine.StatusFailed {
		t.Fatalf("expected failed, got %q", status)
	}
}

func TestUnhealthyStartTriggersCleanup(t *testing.T) {
	a := &fakeAdapter{health: engine.HealthReport{Status: engine.StatusFailed}}
	m, _ := New(a)
	if err := m.Start(context.Background(), nil); err == nil {
		t.Fatal("expected unhealthy start to fail")
	}
	if !a.stopped {
		t.Fatal("expected cleanup stop after unhealthy start")
	}
	status, _ := m.Status()
	if status != engine.StatusFailed {
		t.Fatalf("expected failed, got %q", status)
	}
}

func TestHealthErrorTriggersCleanup(t *testing.T) {
	a := healthyFake()
	a.healthErr = errors.New("probe failed")
	m, _ := New(a)
	if err := m.Start(context.Background(), nil); err == nil {
		t.Fatal("expected health error")
	}
	if !a.stopped {
		t.Fatal("expected cleanup after health error")
	}
}
