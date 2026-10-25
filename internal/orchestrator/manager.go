// Package orchestrator manages one active engine behind the common adapter
// contract. It deliberately does not configure operating-system TUN or DNS.
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
	"github.com/AMINOPY/pg-core/internal/events"
)

const cleanupTimeout = 5 * time.Second

var (
	ErrAlreadyRunning  = errors.New("engine is already running or starting")
	ErrNotRunning      = errors.New("engine is not running")
	ErrContextRequired = errors.New("context is required")
)

type Manager struct {
	adapter engine.Adapter
	opMu    sync.Mutex
	mu      sync.RWMutex
	status  engine.Status
	last    engine.HealthReport
	bus     *events.Bus
}

func New(adapter engine.Adapter) (*Manager, error) {
	return NewWithEvents(adapter, nil)
}

// NewWithEvents optionally publishes lifecycle transitions to an in-process bus.
func NewWithEvents(adapter engine.Adapter, bus *events.Bus) (*Manager, error) {
	if adapter == nil {
		return nil, errors.New("engine adapter is required")
	}
	if adapter.ID() == "" {
		return nil, errors.New("engine adapter ID must not be empty")
	}
	return &Manager{adapter: adapter, status: engine.StatusStopped, bus: bus}, nil
}

func (m *Manager) Status() (engine.Status, engine.HealthReport) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status, m.last
}

func (m *Manager) setStatus(status engine.Status, report engine.HealthReport) {
	m.mu.Lock()
	m.status = status
	if report.EngineID != "" || !report.CheckedAt.IsZero() {
		m.last = report
	}
	m.mu.Unlock()
	if m.bus != nil {
		engineID := report.EngineID
		if engineID == "" {
			engineID = m.adapter.ID()
		}
		m.bus.Publish(events.Event{
			Type:      string(status),
			EngineID:  engineID,
			Status:    string(status),
			ErrorCode: report.ErrorCode,
			At:        time.Now().UTC(),
		})
	}
}

func (m *Manager) cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	return m.adapter.Stop(ctx)
}

func (m *Manager) Start(ctx context.Context, cfg []byte) error {
	if ctx == nil {
		return ErrContextRequired
	}
	m.opMu.Lock()
	defer m.opMu.Unlock()

	m.mu.RLock()
	current := m.status
	m.mu.RUnlock()
	if current == engine.StatusStarting || current == engine.StatusHealthy || current == engine.StatusDegraded {
		return ErrAlreadyRunning
	}

	m.setStatus(engine.StatusStarting, engine.HealthReport{})
	if err := m.adapter.ValidateConfig(ctx, cfg); err != nil {
		m.setStatus(engine.StatusFailed, engine.HealthReport{
			EngineID: m.adapter.ID(), Status: engine.StatusFailed,
			CheckedAt: time.Now().UTC(), ErrorCode: "config_invalid",
		})
		return fmt.Errorf("validate engine config: %w", err)
	}
	if err := ctx.Err(); err != nil {
		m.setStatus(engine.StatusFailed, engine.HealthReport{
			EngineID: m.adapter.ID(), Status: engine.StatusFailed,
			CheckedAt: time.Now().UTC(), ErrorCode: "start_cancelled",
		})
		return err
	}
	if err := m.adapter.Start(ctx, cfg); err != nil {
		cleanupErr := m.cleanup()
		m.setStatus(engine.StatusFailed, engine.HealthReport{
			EngineID: m.adapter.ID(), Status: engine.StatusFailed,
			CheckedAt: time.Now().UTC(), ErrorCode: "start_failed",
		})
		if cleanupErr != nil {
			return fmt.Errorf("start engine: %v; cleanup failed: %w", err, cleanupErr)
		}
		return fmt.Errorf("start engine: %w", err)
	}

	report, err := m.adapter.Health(ctx)
	if err != nil {
		cleanupErr := m.cleanup()
		m.setStatus(engine.StatusFailed, engine.HealthReport{
			EngineID: m.adapter.ID(), Status: engine.StatusFailed,
			CheckedAt: time.Now().UTC(), ErrorCode: "health_check_failed",
		})
		if cleanupErr != nil {
			return fmt.Errorf("health check failed: %v; cleanup failed: %w", err, cleanupErr)
		}
		return fmt.Errorf("health check failed: %w", err)
	}
	if report.EngineID == "" {
		report.EngineID = m.adapter.ID()
	}
	if report.CheckedAt.IsZero() {
		report.CheckedAt = time.Now().UTC()
	}
	if report.Status != engine.StatusHealthy && report.Status != engine.StatusDegraded {
		cleanupErr := m.cleanup()
		m.setStatus(engine.StatusFailed, report)
		if cleanupErr != nil {
			return fmt.Errorf("engine unhealthy after start (status %q); cleanup failed: %w", report.Status, cleanupErr)
		}
		return fmt.Errorf("engine unhealthy after start (status %q)", report.Status)
	}
	m.setStatus(report.Status, report)
	return nil
}

func (m *Manager) RefreshHealth(ctx context.Context) (engine.HealthReport, error) {
	if ctx == nil {
		return engine.HealthReport{}, ErrContextRequired
	}
	m.opMu.Lock()
	defer m.opMu.Unlock()

	m.mu.RLock()
	current := m.status
	m.mu.RUnlock()
	if current != engine.StatusHealthy && current != engine.StatusDegraded {
		return engine.HealthReport{}, ErrNotRunning
	}

	report, err := m.adapter.Health(ctx)
	if err != nil {
		return engine.HealthReport{}, fmt.Errorf("query engine health: %w", err)
	}
	if report.EngineID == "" {
		report.EngineID = m.adapter.ID()
	}
	if report.CheckedAt.IsZero() {
		report.CheckedAt = time.Now().UTC()
	}
	if report.Status != engine.StatusHealthy && report.Status != engine.StatusDegraded {
		report.Status = engine.StatusFailed
		if report.ErrorCode == "" {
			report.ErrorCode = "health_unhealthy"
		}
		cleanupErr := m.cleanup()
		m.setStatus(report.Status, report)
		if cleanupErr != nil {
			return report, fmt.Errorf("engine unhealthy; cleanup failed: %w", cleanupErr)
		}
		return report, nil
	}
	m.setStatus(report.Status, report)
	return report, nil
}

func (m *Manager) Stop(ctx context.Context) error {
	if ctx == nil {
		return ErrContextRequired
	}
	m.opMu.Lock()
	defer m.opMu.Unlock()

	m.mu.RLock()
	current := m.status
	m.mu.RUnlock()
	if current == engine.StatusStopped {
		return nil
	}
	if err := m.adapter.Stop(ctx); err != nil {
		m.setStatus(engine.StatusFailed, engine.HealthReport{
			EngineID: m.adapter.ID(), Status: engine.StatusFailed,
			CheckedAt: time.Now().UTC(), ErrorCode: "stop_failed",
		})
		return fmt.Errorf("stop engine: %w", err)
	}
	m.setStatus(engine.StatusStopped, engine.HealthReport{
		EngineID: m.adapter.ID(), Status: engine.StatusStopped, CheckedAt: time.Now().UTC(),
	})
	return nil
}
