package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/AMINOPY/pg-core/internal/config"
	"github.com/AMINOPY/pg-core/internal/engine"
	"github.com/AMINOPY/pg-core/internal/orchestrator"
	"github.com/AMINOPY/pg-core/internal/policy"
)

type smokeAdapter struct {
	started bool
	stopped bool
}

func (*smokeAdapter) ID() string { return "sing-box" }

func (*smokeAdapter) Capabilities() engine.Capabilities {
	return engine.Capabilities{HealthChecks: true}
}

func (*smokeAdapter) ValidateConfig(_ context.Context, raw []byte) error {
	var cfg config.Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return err
	}
	return cfg.Validate()
}

func (a *smokeAdapter) Start(context.Context, []byte) error {
	a.started = true
	return nil
}

func (*smokeAdapter) Health(context.Context) (engine.HealthReport, error) {
	return engine.HealthReport{
		Status: engine.StatusHealthy,
		Latency: 12 * time.Millisecond,
		CheckedAt: time.Now().UTC(),
	}, nil
}

func (a *smokeAdapter) Stop(context.Context) error {
	a.stopped = true
	return nil
}

// TestCoreControlPlaneSmoke covers config validation, lifecycle orchestration,
// health reporting, deterministic candidate selection, and clean shutdown.
// It uses a fake adapter; it does not claim to test a real VPN data plane.
func TestCoreControlPlaneSmoke(t *testing.T) {
	raw, err := json.Marshal(config.Config{
		SchemaVersion: config.CurrentSchemaVersion,
		Profile:       "smoke",
		ActiveEngine:  config.EngineSingBox,
		Enabled:       []config.EngineID{config.EngineSingBox},
	})
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate config: %v", err)
	}

	adapter := &smokeAdapter{}
	manager, err := orchestrator.New(adapter)
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	if err := manager.Start(context.Background(), raw); err != nil {
		t.Fatalf("start engine lifecycle: %v", err)
	}
	if !adapter.started {
		t.Fatal("expected adapter to start")
	}
	status, report := manager.Status()
	if status != engine.StatusHealthy || report.EngineID != "sing-box" {
		t.Fatalf("unexpected running state: status=%q report=%+v", status, report)
	}

	selected, err := policy.Select([]policy.Candidate{
		{ID: "slow", Status: engine.StatusHealthy, Latency: 80 * time.Millisecond},
		{ID: "fast", Status: engine.StatusHealthy, Latency: 12 * time.Millisecond},
		{ID: "degraded", Status: engine.StatusDegraded, Latency: time.Millisecond},
	})
	if err != nil {
		t.Fatalf("select candidate: %v", err)
	}
	if selected.ID != "fast" {
		t.Fatalf("expected fastest healthy candidate, got %q", selected.ID)
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop engine lifecycle: %v", err)
	}
	if !adapter.stopped {
		t.Fatal("expected adapter to stop")
	}
	status, _ = manager.Status()
	if status != engine.StatusStopped {
		t.Fatalf("expected stopped state, got %q", status)
	}
}
