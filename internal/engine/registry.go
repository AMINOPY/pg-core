package engine

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	ErrDuplicateAdapter = errors.New("engine adapter already registered")
	ErrAdapterNotFound = errors.New("engine adapter not found")
)

type Requirements struct {
	TUN          bool
	DNS          bool
	Routing      bool
	HealthChecks bool
}

// Registry provides a concurrency-safe index of available adapters. Registration
// does not start engines and does not imply that a registered adapter is usable
// on every operating system.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]Adapter
}

func NewRegistry() *Registry {
	return &Registry{adapters: make(map[string]Adapter)}
}

func (r *Registry) Register(adapter Adapter) error {
	if r == nil || adapter == nil {
		return errors.New("registry and adapter are required")
	}
	id := strings.TrimSpace(adapter.ID())
	if id == "" {
		return errors.New("engine adapter ID must not be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.adapters[id]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateAdapter, id)
	}
	r.adapters[id] = adapter
	return nil
}

func (r *Registry) Get(id string) (Adapter, error) {
	if r == nil {
		return nil, errors.New("registry is nil")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrAdapterNotFound, id)
	}
	return adapter, nil
}

func (r *Registry) List() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.adapters))
	for id := range r.adapters {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (r *Registry) Negotiate(id string, required Requirements) error {
	adapter, err := r.Get(id)
	if err != nil {
		return err
	}
	got := adapter.Capabilities()
	var missing []string
	if required.TUN && !got.TUN { missing = append(missing, "tun") }
	if required.DNS && !got.DNS { missing = append(missing, "dns") }
	if required.Routing && !got.Routing { missing = append(missing, "routing") }
	if required.HealthChecks && !got.HealthChecks { missing = append(missing, "health_checks") }
	if len(missing) > 0 {
		return fmt.Errorf("engine %q lacks required capabilities: %s", id, strings.Join(missing, ", "))
	}
	return nil
}
