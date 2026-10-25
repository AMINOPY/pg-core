// Package orchestrator coordinates independent engine managers without merging
// their configurations or silently taking ownership of host networking resources.
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/AMINOPY/pg-core/internal/engine"
)

var (
	ErrEngineNotRegistered = errors.New("engine is not registered with fleet")
	ErrResourceConflict    = errors.New("exclusive host resource is already claimed")
	ErrInvalidResource     = errors.New("resource claim must be a non-empty identifier")
)

type fleetEntry struct {
	manager *Manager
	claims  map[string]struct{}
	active  bool
}

// Fleet manages independent engine lifecycles. Claims describe exclusive host
// resources actually used by a profile (for example "tun:default" or
// "dns:system"); they are not inferred from engine capabilities. The host app
// must provide accurate claims for its configuration.
type Fleet struct {
	opMu sync.Mutex
	mu   sync.RWMutex
	byID map[string]*fleetEntry
}

func NewFleet() *Fleet {
	return &Fleet{byID: make(map[string]*fleetEntry)}
}

// Register adds an engine without starting it. IDs must be unique.
func (f *Fleet) Register(adapter engine.Adapter) error {
	if f == nil {
		return errors.New("fleet is required")
	}
	manager, err := New(adapter)
	if err != nil {
		return err
	}
	id := adapter.ID()
	f.opMu.Lock()
	defer f.opMu.Unlock()
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.byID[id]; exists {
		return fmt.Errorf("%w: %s", engine.ErrDuplicateAdapter, id)
	}
	f.byID[id] = &fleetEntry{manager: manager, claims: make(map[string]struct{})}
	return nil
}

// Start starts one engine while preserving other engine states. Resource claims
// are exclusive across active engines. Empty claims are allowed for process-only
// engines that do not manage shared host resources.
func (f *Fleet) Start(ctx context.Context, id string, cfg []byte, claims ...string) error {
	if f == nil {
		return errors.New("fleet is required")
	}
	if ctx == nil {
		return ErrContextRequired
	}
	normalized, err := normalizeClaims(claims)
	if err != nil {
		return err
	}
	f.opMu.Lock()
	defer f.opMu.Unlock()

	f.mu.RLock()
	entry, ok := f.byID[id]
	if !ok {
		f.mu.RUnlock()
		return fmt.Errorf("%w: %s", ErrEngineNotRegistered, id)
	}
	// A failed health check or failed Stop does not release the engine's
	// resources. Never call Start again until Stop succeeds and clears active.
	if entry.active {
		f.mu.RUnlock()
		return fmt.Errorf("%w: %s still owns its resource claims; stop it successfully before restarting", ErrAlreadyRunning, id)
	}
	for otherID, other := range f.byID {
		if otherID == id {
			continue
		}
		// Keep claims reserved until Fleet.Stop succeeds. A failed health check
		// does not prove that external TUN, DNS, or route state was released.
		if !other.active {
			continue
		}
		for claim := range normalized {
			if _, exists := other.claims[claim]; exists {
				f.mu.RUnlock()
				return fmt.Errorf("%w: %q held by engine %q", ErrResourceConflict, claim, otherID)
			}
		}
	}
	f.mu.RUnlock()

	if err := entry.manager.Start(ctx, cfg); err != nil {
		return err
	}
	f.mu.Lock()
	entry.claims = normalized
	entry.active = true
	f.mu.Unlock()
	return nil
}

// Stop stops only the named engine; unrelated engines remain untouched.
func (f *Fleet) Stop(ctx context.Context, id string) error {
	if f == nil {
		return errors.New("fleet is required")
	}
	if ctx == nil {
		return ErrContextRequired
	}
	f.opMu.Lock()
	defer f.opMu.Unlock()
	f.mu.RLock()
	entry, ok := f.byID[id]
	f.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrEngineNotRegistered, id)
	}
	if err := entry.manager.Stop(ctx); err != nil {
		return err
	}
	f.mu.Lock()
	entry.claims = make(map[string]struct{})
	entry.active = false
	f.mu.Unlock()
	return nil
}

// RefreshHealth runs the adapter's bounded health check. Resource claims remain
// reserved after an unhealthy result until Fleet.Stop succeeds, because health
// failure alone cannot prove that host-level resources have been released.
func (f *Fleet) RefreshHealth(ctx context.Context, id string) (engine.HealthReport, error) {
	if f == nil {
		return engine.HealthReport{}, errors.New("fleet is required")
	}
	if ctx == nil {
		return engine.HealthReport{}, ErrContextRequired
	}
	f.opMu.Lock()
	defer f.opMu.Unlock()
	f.mu.RLock()
	entry, ok := f.byID[id]
	f.mu.RUnlock()
	if !ok {
		return engine.HealthReport{}, fmt.Errorf("%w: %s", ErrEngineNotRegistered, id)
	}
	return entry.manager.RefreshHealth(ctx)
}

// Status returns the named engine's last lifecycle state.
func (f *Fleet) Status(id string) (engine.Status, engine.HealthReport, error) {
	if f == nil {
		return "", engine.HealthReport{}, errors.New("fleet is required")
	}
	f.mu.RLock()
	entry, ok := f.byID[id]
	f.mu.RUnlock()
	if !ok {
		return "", engine.HealthReport{}, fmt.Errorf("%w: %s", ErrEngineNotRegistered, id)
	}
	status, report := entry.manager.Status()
	return status, report, nil
}

// List returns the registered engine IDs in stable lexical order.
func (f *Fleet) List() []string {
	if f == nil {
		return nil
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	ids := make([]string, 0, len(f.byID))
	for id := range f.byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func normalizeClaims(claims []string) (map[string]struct{}, error) {
	normalized := make(map[string]struct{}, len(claims))
	for _, claim := range claims {
		claim = strings.TrimSpace(claim)
		if claim == "" || strings.ContainsAny(claim, "\r\n") {
			return nil, ErrInvalidResource
		}
		normalized[claim] = struct{}{}
	}
	return normalized, nil
}
