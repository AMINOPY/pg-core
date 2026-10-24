package integration

import (
	"context"
	"errors"
	"testing"
)

type testRegistry struct {
	actions []Action
	failAt  int
}

func (r *testRegistry) RegisterAction(a Action) error {
	if r.failAt > 0 && len(r.actions)+1 == r.failAt {
		return errors.New("navigation adapter unavailable")
	}
	r.actions = append(r.actions, a)
	return nil
}

type testModule struct {
	desc Descriptor
	opened EntryPoint
}

func (m *testModule) Descriptor() Descriptor { return m.desc }
func (m *testModule) Open(_ context.Context, entry EntryPoint, _ HostContext) error {
	m.opened = entry
	return nil
}

func TestRegisterPublishesBothHostEntryPoints(t *testing.T) {
	reg := &testRegistry{}
	mod := &testModule{desc: Descriptor{
		ID: "transport.options", Title: "Transport options",
		EntryPoints: []EntryPoint{EntrySettings, EntryAddConnection},
	}}
	if err := Register(reg, mod); err != nil {
		t.Fatal(err)
	}
	if len(reg.actions) != 2 {
		t.Fatalf("got %d actions, want 2", len(reg.actions))
	}
	if reg.actions[0].EntryPoint != EntrySettings || reg.actions[1].EntryPoint != EntryAddConnection {
		t.Fatalf("unexpected entry points: %#v", reg.actions)
	}
}

func TestRegisterRejectsInvalidDescriptor(t *testing.T) {
	cases := []Descriptor{
		{ID: "../bad", Title: "Bad", EntryPoints: []EntryPoint{EntrySettings}},
		{ID: "valid", Title: " ", EntryPoints: []EntryPoint{EntrySettings}},
		{ID: "valid", Title: "Valid"},
		{ID: "valid", Title: "Valid", EntryPoints: []EntryPoint{"home"}},
		{ID: "valid", Title: "Valid", EntryPoints: []EntryPoint{EntrySettings, EntrySettings}},
	}
	for _, desc := range cases {
		if err := Register(&testRegistry{}, &testModule{desc: desc}); err == nil {
			t.Errorf("Register(%#v) succeeded; want validation error", desc)
		}
	}
}

func TestRegisterPropagatesHostAdapterError(t *testing.T) {
	reg := &testRegistry{failAt: 1}
	mod := &testModule{desc: Descriptor{ID: "sample", Title: "Sample", EntryPoints: []EntryPoint{EntrySettings}}}
	if err := Register(reg, mod); err == nil {
		t.Fatal("expected host adapter error")
	}
}

func TestInvokeOnlyAllowsDeclaredEntryPoint(t *testing.T) {
	mod := &testModule{desc: Descriptor{ID: "sample", Title: "Sample", EntryPoints: []EntryPoint{EntrySettings}}}
	if err := Invoke(context.Background(), mod, EntryAddConnection, HostContext{}); !errors.Is(err, ErrUnknownEntryPoint) {
		t.Fatalf("got %v, want ErrUnknownEntryPoint", err)
	}
	if err := Invoke(context.Background(), mod, EntrySettings, HostContext{AppID: "host-app"}); err != nil {
		t.Fatal(err)
	}
	if mod.opened != EntrySettings {
		t.Fatalf("opened %q, want settings", mod.opened)
	}
}

func TestInvokeHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mod := &testModule{desc: Descriptor{ID: "sample", Title: "Sample", EntryPoints: []EntryPoint{EntrySettings}}}
	if err := Invoke(ctx, mod, EntrySettings, HostContext{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}
