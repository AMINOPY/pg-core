package engine

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type testAdapter struct { id string; caps Capabilities }
func (a testAdapter) ID() string { return a.id }
func (a testAdapter) Capabilities() Capabilities { return a.caps }
func (a testAdapter) ValidateConfig(context.Context, []byte) error { return nil }
func (a testAdapter) Start(context.Context, []byte) error { return nil }
func (a testAdapter) Health(context.Context) (HealthReport, error) { return HealthReport{Status: StatusHealthy, CheckedAt: time.Now()}, nil }
func (a testAdapter) Stop(context.Context) error { return nil }

func TestRegistryRegisterListGetAndNegotiate(t *testing.T) {
	r := NewRegistry()
	a := testAdapter{id:"alpha", caps:Capabilities{TUN:true, DNS:true, HealthChecks:true}}
	if err := r.Register(a); err != nil { t.Fatal(err) }
	if err := r.Register(a); !errors.Is(err, ErrDuplicateAdapter) { t.Fatalf("expected duplicate error, got %v", err) }
	if got := r.List(); !reflect.DeepEqual(got, []string{"alpha"}) { t.Fatalf("List() = %v", got) }
	if _, err := r.Get("missing"); !errors.Is(err, ErrAdapterNotFound) { t.Fatalf("Get missing: %v", err) }
	if err := r.Negotiate("alpha", Requirements{TUN:true, DNS:true}); err != nil { t.Fatal(err) }
	if err := r.Negotiate("alpha", Requirements{TUN:true, Routing:true}); err == nil { t.Fatal("expected missing capability error") }
}
