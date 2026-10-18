// Package fusion defines a stable adapter contract for composing heterogeneous
// network engines without assuming that their internal APIs are interchangeable.
package fusion

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

var (
	ErrDuplicateEngine = errors.New("fusion: engine already registered")
	ErrEngineNotFound  = errors.New("fusion: engine not found")
	ErrExclusiveOwner  = errors.New("fusion: exclusive network resource already owned")
)

type Capability string

const (
	CapabilityProxy  Capability = "proxy"
	CapabilityDNS    Capability = "dns"
	CapabilityTUN    Capability = "tun"
	CapabilityHealth Capability = "health"
	CapabilityReload Capability = "reload"
)

type Descriptor struct {
	ID                 string
	Version            string
	Capabilities       []Capability
	ExclusiveResources []string
}

type Engine interface {
	Descriptor() Descriptor
	Start(context.Context) error
	Stop(context.Context) error
	Health(context.Context) error
}

type Registry struct {
	mu            sync.RWMutex
	engines       map[string]Engine
	running       map[string]bool
	resourceOwner map[string]string
}

func NewRegistry() *Registry {
	return &Registry{
		engines:       map[string]Engine{},
		running:       map[string]bool{},
		resourceOwner: map[string]string{},
	}
}

func (r *Registry) Register(engine Engine) error {
	if engine == nil {
		return errors.New("fusion: nil engine")
	}
	d := engine.Descriptor()
	if d.ID == "" {
		return errors.New("fusion: engine ID is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.engines[d.ID]; ok {
		return fmt.Errorf("%w: %s", ErrDuplicateEngine, d.ID)
	}
	r.engines[d.ID] = engine
	return nil
}

func (r *Registry) List() []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.engines))
	for id := range r.engines {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]Descriptor, 0, len(ids))
	for _, id := range ids {
		out = append(out, r.engines[id].Descriptor())
	}
	return out
}

func (r *Registry) Start(ctx context.Context, id string) error {
	r.mu.Lock()
	engine, ok := r.engines[id]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrEngineNotFound, id)
	}
	if r.running[id] {
		r.mu.Unlock()
		return nil
	}
	d := engine.Descriptor()
	for _, resource := range d.ExclusiveResources {
		if owner, used := r.resourceOwner[resource]; used && owner != id {
			r.mu.Unlock()
			return fmt.Errorf("%w: resource %q held by %q", ErrExclusiveOwner, resource, owner)
		}
	}
	for _, resource := range d.ExclusiveResources {
		r.resourceOwner[resource] = id
	}
	r.mu.Unlock()

	if err := engine.Start(ctx); err != nil {
		r.mu.Lock()
		for _, resource := range d.ExclusiveResources {
			if r.resourceOwner[resource] == id {
				delete(r.resourceOwner, resource)
			}
		}
		r.mu.Unlock()
		return fmt.Errorf("fusion: start %s: %w", id, err)
	}
	r.mu.Lock()
	r.running[id] = true
	r.mu.Unlock()
	return nil
}

func (r *Registry) Stop(ctx context.Context, id string) error {
	r.mu.RLock()
	engine, ok := r.engines[id]
	running := r.running[id]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrEngineNotFound, id)
	}
	if !running {
		return nil
	}
	if err := engine.Stop(ctx); err != nil {
		return fmt.Errorf("fusion: stop %s: %w", id, err)
	}
	r.mu.Lock()
	delete(r.running, id)
	for _, resource := range engine.Descriptor().ExclusiveResources {
		if r.resourceOwner[resource] == id {
			delete(r.resourceOwner, resource)
		}
	}
	r.mu.Unlock()
	return nil
}

func (r *Registry) Health(ctx context.Context, id string) error {
	r.mu.RLock()
	engine, ok := r.engines[id]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrEngineNotFound, id)
	}
	return engine.Health(ctx)
}
