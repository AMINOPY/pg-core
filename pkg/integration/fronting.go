package integration

import (
	"context"
	"errors"
)

// OpenFunc is implemented by the embedding application to navigate to its
// native Fronting CDN/SNI screen and bind it to the engine/profile it supports.
// The callback owns engine capability checks and all configuration persistence.
type OpenFunc func(context.Context, EntryPoint, HostContext) error

type callbackModule struct {
	open OpenFunc
}

// NewFrontingModule creates the standard CDN/SNI integration entry for host
// apps that embed PG CORE alongside other engines. The same module is exposed
// in Settings and Add Connection; the host decides whether it is available for
// the currently selected engine.
func NewFrontingModule(open OpenFunc) Module {
	return &callbackModule{open: open}
}

func (m *callbackModule) Descriptor() Descriptor {
	return Descriptor{
		ID:          "fronting.cdn-sni",
		Title:       "Fronting CDN/SNI",
		Description: "Configure SNI and CDN edge settings for a supported connection",
		EntryPoints: []EntryPoint{EntrySettings, EntryAddConnection},
	}
}

func (m *callbackModule) Open(ctx context.Context, entry EntryPoint, host HostContext) error {
	if m == nil || m.open == nil {
		return errors.New("integration: Fronting CDN/SNI host callback is required")
	}
	if ctx == nil {
		return errors.New("integration: context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if entry != EntrySettings && entry != EntryAddConnection {
		return ErrUnknownEntryPoint
	}
	return m.open(ctx, entry, host)
}
