package engine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var (
	ErrCoordinatorRunning = errors.New("engine coordinator is already running")
	ErrCoordinatorStopped = errors.New("engine coordinator is not running")
)

const rollbackTimeout = 5 * time.Second

// EnginePlan is an explicit request to start one registered adapter. Config is
// copied by the coordinator and must never be logged by adapters or callers.
type EnginePlan struct {
	ID           string
	Config       []byte
	Requirements Requirements
}

// EngineState is a secret-free snapshot suitable for status/UI reporting.
type EngineState struct {
	ID     string
	Status Status
	Error  string
}

// Coordinator owns the lifecycle of a selected set of adapters. It serializes
// start/stop, validates every config before starting any process, checks health
// before reporting success, and rolls back partially started plans on failure.
// It does not create a TUN device or claim system-wide VPN routing.
type Coordinator struct {
	mu       sync.Mutex
	registry *Registry
	running  []Adapter
	states   map[string]EngineState
}

func NewCoordinator(registry *Registry) (*Coordinator, error) {
	if registry == nil {
		return nil, errors.New("engine registry is required")
	}
	return &Coordinator{registry: registry, states: make(map[string]EngineState)}, nil
}

// Start validates all plans before starting any engine. All selected engines
// must pass their bounded Health check or all engines started by this call are
// stopped in reverse order. Callers should supply a deadline on ctx.
func (c *Coordinator) Start(ctx context.Context, plans []EnginePlan) error {
	if c == nil {
		return errors.New("engine coordinator is required")
	}
	if ctx == nil {
		return errors.New("context is required")
	}
	if len(plans) == 0 {
		return errors.New("at least one engine plan is required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.running) > 0 {
		return ErrCoordinatorRunning
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	ordered := append([]EnginePlan(nil), plans...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	adapters := make([]Adapter, len(ordered))
	seen := make(map[string]struct{}, len(ordered))
	for i, plan := range ordered {
		if plan.ID == "" {
			return errors.New("engine plan ID is required")
		}
		if _, ok := seen[plan.ID]; ok {
			return fmt.Errorf("duplicate engine plan %q", plan.ID)
		}
		seen[plan.ID] = struct{}{}
		adapter, err := c.registry.Get(plan.ID)
		if err != nil {
			return err
		}
		if err := c.registry.Negotiate(plan.ID, plan.Requirements); err != nil {
			return err
		}
		if len(plan.Config) == 0 {
			return fmt.Errorf("engine %q config is empty", plan.ID)
		}
		if err := adapter.ValidateConfig(ctx, append([]byte(nil), plan.Config...)); err != nil {
			return fmt.Errorf("validate engine %q config: %w", plan.ID, err)
		}
		adapters[i] = adapter
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	started := make([]Adapter, 0, len(adapters))
	for i, adapter := range adapters {
		plan := ordered[i]
		if err := ctx.Err(); err != nil {
			c.running = c.rollback(started)
			return err
		}
		c.states[plan.ID] = EngineState{ID: plan.ID, Status: StatusStarting}
		if err := adapter.Start(ctx, append([]byte(nil), plan.Config...)); err != nil {
			c.states[plan.ID] = EngineState{ID: plan.ID, Status: StatusFailed, Error: "start_failed"}
			c.running = c.rollback(append(started, adapter))
			return fmt.Errorf("start engine %q: %w", plan.ID, err)
		}
		started = append(started, adapter)
		report, err := adapter.Health(ctx)
		if err != nil || report.Status != StatusHealthy {
			c.states[plan.ID] = EngineState{ID: plan.ID, Status: StatusDegraded, Error: "health_check_failed"}
			c.running = c.rollback(started)
			if err != nil {
				return fmt.Errorf("health check engine %q: %w", plan.ID, err)
			}
			return fmt.Errorf("health check engine %q returned status %q", plan.ID, report.Status)
		}
		c.states[plan.ID] = EngineState{ID: plan.ID, Status: StatusHealthy}
	}
	c.running = started
	return nil
}

func (c *Coordinator) rollback(started []Adapter) []Adapter {
	ctx, cancel := context.WithTimeout(context.Background(), rollbackTimeout)
	defer cancel()
	remaining := make([]Adapter, 0)
	for i := len(started) - 1; i >= 0; i-- {
		adapter := started[i]
		if err := adapter.Stop(ctx); err != nil {
			c.states[adapter.ID()] = EngineState{ID: adapter.ID(), Status: StatusFailed, Error: "rollback_stop_failed"}
			remaining = append(remaining, adapter)
			continue
		}
		c.states[adapter.ID()] = EngineState{ID: adapter.ID(), Status: StatusStopped}
	}
	return remaining
}

// Stop attempts every stop in reverse startup order. Failed stops remain marked
// failed and are retained as owned so callers can retry rather than silently
// losing lifecycle ownership.
func (c *Coordinator) Stop(ctx context.Context) error {
	if c == nil {
		return errors.New("engine coordinator is required")
	}
	if ctx == nil {
		return errors.New("context is required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.running) == 0 {
		return ErrCoordinatorStopped
	}
	remaining := make([]Adapter, 0, len(c.running))
	var failures []error
	for i := len(c.running) - 1; i >= 0; i-- {
		adapter := c.running[i]
		if err := adapter.Stop(ctx); err != nil {
			failures = append(failures, fmt.Errorf("stop engine %q: %w", adapter.ID(), err))
			c.states[adapter.ID()] = EngineState{ID: adapter.ID(), Status: StatusFailed, Error: "stop_failed"}
			remaining = append(remaining, adapter)
			continue
		}
		c.states[adapter.ID()] = EngineState{ID: adapter.ID(), Status: StatusStopped}
	}
	c.running = remaining
	return errors.Join(failures...)
}

// Snapshot returns sorted, secret-free state values.
func (c *Coordinator) Snapshot() []EngineState {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ids := make([]string, 0, len(c.states))
	for id := range c.states {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]EngineState, 0, len(ids))
	for _, id := range ids {
		out = append(out, c.states[id])
	}
	return out
}
