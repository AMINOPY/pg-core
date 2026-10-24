// Package integration defines a stable, UI-agnostic extension contract for host
// applications that embed PG CORE alongside one or more network engines.
//
// This package deliberately does not import internal engine packages. Host apps
// can register actions in their Settings and Add Connection surfaces, then route
// an action to an app-owned module/adapter appropriate for the selected engine.
package integration

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type EntryPoint string

const (
	EntrySettings     EntryPoint = "settings"
	EntryAddConnection EntryPoint = "add_connection"
)

// Descriptor is the user-visible metadata for a module. IDs must be stable,
// lowercase identifiers (for example, "transport.options").
type Descriptor struct {
	ID          string
	Title       string
	Description string
	EntryPoints []EntryPoint
}

// Action is a render-neutral item a host UI can place in a menu, Settings page,
// or Add Connection flow. The host owns actual rendering and navigation.
type Action struct {
	ModuleID    string
	Title       string
	Description string
	EntryPoint  EntryPoint
}

// HostContext contains non-secret runtime context supplied by the host. Do not
// put credentials, tokens, private keys, or full configuration blobs here.
type HostContext struct {
	AppID             string
	ActiveEngineID    string
	AvailableEngineIDs []string
}

// Module implements a capability that can be opened from one or more host UI
// entry points. Implementations should return promptly on context cancellation.
type Module interface {
	Descriptor() Descriptor
	Open(ctx context.Context, entry EntryPoint, host HostContext) error
}

// Registry is implemented by each host app's UI/navigation adapter. It may map
// actions to native Android, desktop, Flutter, or web navigation.
type Registry interface {
	RegisterAction(Action) error
}

var (
	ErrNilRegistry = errors.New("integration: registry is required")
	ErrNilModule   = errors.New("integration: module is required")
	ErrUnknownEntryPoint = errors.New("integration: unsupported entry point")
)

func Register(registry Registry, module Module) error {
	if registry == nil {
		return ErrNilRegistry
	}
	if module == nil {
		return ErrNilModule
	}
	d := module.Descriptor()
	if !validID(d.ID) {
		return errors.New("integration: module ID must be a stable lowercase identifier")
	}
	if strings.TrimSpace(d.Title) == "" {
		return errors.New("integration: module title is required")
	}
	if len(d.EntryPoints) == 0 {
		return errors.New("integration: at least one entry point is required")
	}

	seen := make(map[EntryPoint]bool, len(d.EntryPoints))
	for _, entry := range d.EntryPoints {
		if entry != EntrySettings && entry != EntryAddConnection {
			return fmt.Errorf("%w: %q", ErrUnknownEntryPoint, entry)
		}
		if seen[entry] {
			return fmt.Errorf("integration: duplicate entry point %q", entry)
		}
		seen[entry] = true
		action := Action{
			ModuleID: d.ID, Title: d.Title, Description: d.Description,
			EntryPoint: entry,
		}
		if err := registry.RegisterAction(action); err != nil {
			return fmt.Errorf("integration: register %s action: %w", entry, err)
		}
	}
	return nil
}

// Invoke validates that the requested entry point is declared before handing
// control to the module. It does not select or start a network engine itself.
func Invoke(ctx context.Context, module Module, entry EntryPoint, host HostContext) error {
	if module == nil {
		return ErrNilModule
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	d := module.Descriptor()
	for _, declared := range d.EntryPoints {
		if declared == entry {
			return module.Open(ctx, entry, host)
		}
	}
	return fmt.Errorf("%w: %q", ErrUnknownEntryPoint, entry)
}

func validID(id string) bool {
	if id == "" || strings.TrimSpace(id) != id {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '.' && r != '-' && r != '_' {
			return false
		}
	}
	return true
}
