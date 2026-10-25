package fusion

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

var ErrPathInUse = errors.New("cannot remove a fusion path with active flows")

// Allocator owns path snapshots and active-flow counters. Acquire is atomic with
// selection, so concurrent callers cannot all select a path based on the same
// stale ActiveFlows count. The returned release function is safe to call more
// than once. Actual dialing, health sampling, and connection closure remain the
// caller's responsibility.
type Allocator struct {
	mu    sync.Mutex
	paths map[string]Path
}

// NewAllocator creates an allocator with the supplied initial path snapshots.
func NewAllocator(paths []Path) (*Allocator, error) {
	a := &Allocator{paths: make(map[string]Path)}
	if err := a.Update(paths); err != nil {
		return nil, err
	}
	return a, nil
}

// Update replaces path health/quality snapshots while preserving allocator-owned
// active-flow counts. A path cannot be removed while a lease still references it.
func (a *Allocator) Update(paths []Path) error {
	if a == nil {
		return errors.New("fusion allocator is required")
	}
	// Select validates every field and duplicate ID before returning either a
	// selected path or ErrNoEligiblePath (which is valid for an all-down set).
	if _, err := Select(paths); err != nil && !errors.Is(err, ErrNoEligiblePath) {
		return err
	}

	next := make(map[string]Path, len(paths))
	for _, path := range paths {
		next[path.ID] = path
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for id, current := range a.paths {
		if current.ActiveFlows == 0 {
			continue
		}
		if _, exists := next[id]; !exists {
			return fmt.Errorf("%w: %q has %d active flow(s)", ErrPathInUse, id, current.ActiveFlows)
		}
	}
	for id, path := range next {
		if current, exists := a.paths[id]; exists {
			path.ActiveFlows = current.ActiveFlows
		} else {
			// Counters are owned by this allocator, not by external telemetry.
			path.ActiveFlows = 0
		}
		next[id] = path
	}
	a.paths = next
	return nil
}

// Acquire atomically selects a path and reserves one active-flow slot.
func (a *Allocator) Acquire() (Path, func(), error) {
	if a == nil {
		return Path{}, nil, errors.New("fusion allocator is required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	paths := make([]Path, 0, len(a.paths))
	for _, path := range a.paths {
		paths = append(paths, path)
	}
	selected, err := Select(paths)
	if err != nil {
		return Path{}, nil, err
	}
	selected.ActiveFlows++
	a.paths[selected.ID] = selected

	var released atomic.Bool
	release := func() {
		if !released.CompareAndSwap(false, true) {
			return
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		current, exists := a.paths[selected.ID]
		if exists && current.ActiveFlows > 0 {
			current.ActiveFlows--
			a.paths[selected.ID] = current
		}
	}
	return selected, release, nil
}

// Snapshot returns a stable copy of path snapshots, including allocator-owned
// active-flow counters. It is intended for status display and diagnostics.
func (a *Allocator) Snapshot() []Path {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	paths := make([]Path, 0, len(a.paths))
	for _, path := range a.paths {
		paths = append(paths, path)
	}
	return paths
}
