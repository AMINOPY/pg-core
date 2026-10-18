// Package capability lets Chimera discover reusable engine capabilities and
// decide whether to adopt them as native implementations or delegate via adapters.
// It does not copy arbitrary third-party source code; source reuse must be
// explicit and license-compatible.
package capability

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type AdoptionMode string

const (
	Native AdoptionMode = "native"
	Adapter AdoptionMode = "adapter"
	Unavailable AdoptionMode = "unavailable"
)

type Requirement struct {
	Name string
	MinimumVersion string
}

type Manifest struct {
	ID string
	Version string
	Summary string
	Capabilities []string
	Requires []Requirement
	License string
	SourceURL string
	// SourceReuseAllowed must only be true after a human has verified the
	// source license and compatibility. It is not an automatic license detector.
	SourceReuseAllowed bool
}

type Host struct {
	Capabilities map[string]string
}

type Assessment struct {
	Manifest Manifest
	Mode AdoptionMode
	Missing []string
	Reason string
}

type Catalog struct {
	mu sync.RWMutex
	items map[string]Manifest
}

func NewCatalog() *Catalog { return &Catalog{items: make(map[string]Manifest)} }

func (c *Catalog) Register(m Manifest) error {
	m.ID = strings.TrimSpace(m.ID)
	m.Version = strings.TrimSpace(m.Version)
	if m.ID == "" || m.Version == "" {
		return errors.New("capability: manifest ID and version are required")
	}
	if len(m.Capabilities) == 0 {
		return errors.New("capability: at least one capability is required")
	}
	for _, name := range m.Capabilities {
		if strings.TrimSpace(name) == "" { return errors.New("capability: empty capability name") }
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.items[m.ID]; exists { return fmt.Errorf("capability: duplicate manifest %q", m.ID) }
	m.Capabilities = append([]string(nil), m.Capabilities...)
	m.Requires = append([]Requirement(nil), m.Requires...)
	c.items[m.ID] = m
	return nil
}

func (c *Catalog) List() []Manifest {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ids := make([]string, 0, len(c.items))
	for id := range c.items { ids = append(ids, id) }
	sort.Strings(ids)
	out := make([]Manifest, 0, len(ids))
	for _, id := range ids {
		m := c.items[id]
		m.Capabilities = append([]string(nil), m.Capabilities...)
		m.Requires = append([]Requirement(nil), m.Requires...)
		out = append(out, m)
	}
	return out
}

// Assess determines whether a feature can be adopted using the host's declared
// capabilities. It never executes code or fetches a third-party repository.
func (c *Catalog) Assess(id string, host Host) (Assessment, error) {
	c.mu.RLock()
	m, ok := c.items[id]
	c.mu.RUnlock()
	if !ok { return Assessment{}, fmt.Errorf("capability: manifest %q not found", id) }
	result := Assessment{Manifest:m, Mode:Adapter}
	for _, req := range m.Requires {
		if _, ok := host.Capabilities[req.Name]; !ok {
			result.Missing = append(result.Missing, req.Name)
		}
	}
	if len(result.Missing) > 0 {
		result.Mode = Unavailable
		result.Reason = "host does not provide all required capabilities"
		return result, nil
	}
	if m.SourceReuseAllowed && strings.TrimSpace(m.License) != "" {
		result.Mode = Native
		result.Reason = "source reuse was explicitly approved; review attribution and license obligations"
		return result, nil
	}
	result.Mode = Adapter
	result.Reason = "use the provider adapter; source reuse is not approved"
	return result, nil
}
