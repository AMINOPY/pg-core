package integration

import (
	"context"
	"errors"
	"testing"
)

func TestFrontingModuleRegistersBothEntryPoints(t *testing.T) {
	mod := NewFrontingModule(func(context.Context, EntryPoint, HostContext) error { return nil })
	d := mod.Descriptor()
	if d.ID != "fronting.cdn-sni" {
		t.Fatalf("unexpected module ID %q", d.ID)
	}
	if len(d.EntryPoints) != 2 || d.EntryPoints[0] != EntrySettings || d.EntryPoints[1] != EntryAddConnection {
		t.Fatalf("unexpected entry points: %#v", d.EntryPoints)
	}
	reg := &testRegistry{}
	if err := Register(reg, mod); err != nil {
		t.Fatal(err)
	}
	if len(reg.actions) != 2 {
		t.Fatalf("got %d actions, want 2", len(reg.actions))
	}
}

func TestFrontingModuleDelegatesSelectedEntry(t *testing.T) {
	var called EntryPoint
	mod := NewFrontingModule(func(_ context.Context, entry EntryPoint, host HostContext) error {
		called = entry
		if host.ActiveEngineID != "supported-engine" {
			t.Fatalf("unexpected engine %q", host.ActiveEngineID)
		}
		return nil
	})
	if err := Invoke(context.Background(), mod, EntryAddConnection, HostContext{ActiveEngineID: "supported-engine"}); err != nil {
		t.Fatal(err)
	}
	if called != EntryAddConnection {
		t.Fatalf("called with %q, want add_connection", called)
	}
}

func TestFrontingModuleRejectsMissingCallbackAndCancellation(t *testing.T) {
	if err := NewFrontingModule(nil).Open(context.Background(), EntrySettings, HostContext{}); err == nil {
		t.Fatal("expected missing callback error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mod := NewFrontingModule(func(context.Context, EntryPoint, HostContext) error {
		t.Fatal("callback must not run for canceled context")
		return nil
	})
	if err := mod.Open(ctx, EntrySettings, HostContext{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}
