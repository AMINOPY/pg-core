package fusion

import (
	"context"
	"errors"
	"testing"
)

type fakeEngine struct {
	d        Descriptor
	starts   int
	stops    int
	startErr error
}

func (e *fakeEngine) Descriptor() Descriptor       { return e.d }
func (e *fakeEngine) Start(context.Context) error  { e.starts++; return e.startErr }
func (e *fakeEngine) Stop(context.Context) error   { e.stops++; return nil }
func (e *fakeEngine) Health(context.Context) error { return nil }

func TestRegistryPreventsConflictingTUNOwners(t *testing.T) {
	r := NewRegistry()
	a := &fakeEngine{d: Descriptor{ID: "sing-box", ExclusiveResources: []string{"android-vpn-tun"}}}
	b := &fakeEngine{d: Descriptor{ID: "other-core", ExclusiveResources: []string{"android-vpn-tun"}}}
	if err := r.Register(a); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(b); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background(), a.d.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background(), b.d.ID); !errors.Is(err, ErrExclusiveOwner) {
		t.Fatalf("expected ownership conflict, got %v", err)
	}
	if b.starts != 0 {
		t.Fatal("conflicting engine must not start")
	}
	if err := r.Stop(context.Background(), a.d.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background(), b.d.ID); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryRejectsDuplicateIDs(t *testing.T) {
	r := NewRegistry()
	e := &fakeEngine{d: Descriptor{ID: "core"}}
	if err := r.Register(e); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(e); !errors.Is(err, ErrDuplicateEngine) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}
