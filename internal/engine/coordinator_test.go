package engine

import (
    "context"
    "errors"
    "reflect"
    "testing"
    "time"
)

type coordinatorTestAdapter struct {
    id string
    caps Capabilities
    events *[]string
    validateErr error
    startErr error
    healthStatus Status
    healthErr error
    stopErr error
}

func (a *coordinatorTestAdapter) ID() string { return a.id }
func (a *coordinatorTestAdapter) Capabilities() Capabilities { return a.caps }
func (a *coordinatorTestAdapter) ValidateConfig(context.Context, []byte) error {
    *a.events = append(*a.events, "validate:"+a.id)
    return a.validateErr
}
func (a *coordinatorTestAdapter) Start(context.Context, []byte) error {
    *a.events = append(*a.events, "start:"+a.id)
    return a.startErr
}
func (a *coordinatorTestAdapter) Health(context.Context) (HealthReport, error) {
    *a.events = append(*a.events, "health:"+a.id)
    return HealthReport{EngineID:a.id, Status:a.healthStatus, CheckedAt:time.Now()}, a.healthErr
}
func (a *coordinatorTestAdapter) Stop(context.Context) error {
    *a.events = append(*a.events, "stop:"+a.id)
    return a.stopErr
}

func newCoordinatorTest(t *testing.T, adapters ...Adapter) (*Coordinator, *[]string) {
    t.Helper()
    registry := NewRegistry()
    for _, adapter := range adapters { if err := registry.Register(adapter); err != nil { t.Fatal(err) } }
    c, err := NewCoordinator(registry)
    if err != nil { t.Fatal(err) }
    events := []string{}
    for _, raw := range adapters { raw.(*coordinatorTestAdapter).events = &events }
    return c, &events
}

func TestCoordinatorValidatesAllBeforeStartingAndStopsReverseOrder(t *testing.T) {
    a := &coordinatorTestAdapter{id:"alpha", caps:Capabilities{HealthChecks:true}, healthStatus:StatusHealthy}
    b := &coordinatorTestAdapter{id:"beta", caps:Capabilities{HealthChecks:true}, healthStatus:StatusHealthy}
    c, events := newCoordinatorTest(t, b, a)
    plans := []EnginePlan{{ID:"beta", Config:[]byte(`{"id":"beta"}`)}, {ID:"alpha", Config:[]byte(`{"id":"alpha"}`)}}
    if err := c.Start(context.Background(), plans); err != nil { t.Fatal(err) }
    want := []string{"validate:alpha","validate:beta","start:alpha","health:alpha","start:beta","health:beta"}
    if !reflect.DeepEqual(*events, want) { t.Fatalf("events = %v, want %v", *events, want) }
    if err := c.Stop(context.Background()); err != nil { t.Fatal(err) }
    got := (*events)[len(want):]
    if wantStop := []string{"stop:beta","stop:alpha"}; !reflect.DeepEqual(got, wantStop) { t.Fatalf("stop order = %v, want %v", got, wantStop) }
}

func TestCoordinatorRejectsInvalidPlanBeforeStartingAnything(t *testing.T) {
    a := &coordinatorTestAdapter{id:"alpha", caps:Capabilities{HealthChecks:true}, healthStatus:StatusHealthy}
    b := &coordinatorTestAdapter{id:"beta", caps:Capabilities{HealthChecks:true}, validateErr:errors.New("bad config"), healthStatus:StatusHealthy}
    c, events := newCoordinatorTest(t, a, b)
    err := c.Start(context.Background(), []EnginePlan{{ID:"alpha",Config:[]byte("{}")},{ID:"beta",Config:[]byte("{}")}})
    if err == nil { t.Fatal("expected validation error") }
    for _, event := range *events { if len(event) >= 6 && event[:6] == "start:" { t.Fatalf("engine started before validation completed: %v", *events) } }
}

func TestCoordinatorRollsBackOnStartFailure(t *testing.T) {
    a := &coordinatorTestAdapter{id:"alpha", caps:Capabilities{HealthChecks:true}, healthStatus:StatusHealthy}
    b := &coordinatorTestAdapter{id:"beta", caps:Capabilities{HealthChecks:true}, startErr:errors.New("start failed"), healthStatus:StatusHealthy}
    c, events := newCoordinatorTest(t, a, b)
    err := c.Start(context.Background(), []EnginePlan{{ID:"alpha",Config:[]byte("{}")},{ID:"beta",Config:[]byte("{}")}})
    if err == nil { t.Fatal("expected start error") }
    want := []string{"validate:alpha","validate:beta","start:alpha","health:alpha","start:beta","stop:beta","stop:alpha"}
    if !reflect.DeepEqual(*events, want) { t.Fatalf("events = %v, want %v", *events, want) }
}

func TestCoordinatorRollsBackWhenHealthIsNotHealthy(t *testing.T) {
    a := &coordinatorTestAdapter{id:"alpha", caps:Capabilities{HealthChecks:true}, healthStatus:StatusDegraded}
    c, events := newCoordinatorTest(t, a)
    err := c.Start(context.Background(), []EnginePlan{{ID:"alpha",Config:[]byte("{}")}})
    if err == nil { t.Fatal("expected health gate failure") }
    want := []string{"validate:alpha","start:alpha","health:alpha","stop:alpha"}
    if !reflect.DeepEqual(*events, want) { t.Fatalf("events = %v, want %v", *events, want) }
}

func TestCoordinatorStopFailureRetainsRetryOwnership(t *testing.T) {
    stopErr := errors.New("stop failed")
    a := &coordinatorTestAdapter{id:"alpha", caps:Capabilities{HealthChecks:true}, healthStatus:StatusHealthy, stopErr:stopErr}
    c, _ := newCoordinatorTest(t, a)
    if err := c.Start(context.Background(), []EnginePlan{{ID:"alpha",Config:[]byte("{}")}}); err != nil { t.Fatal(err) }
    if err := c.Stop(context.Background()); !errors.Is(err, stopErr) { t.Fatalf("Stop error = %v, want %v", err, stopErr) }
    if err := c.Stop(context.Background()); !errors.Is(err, stopErr) { t.Fatalf("retry Stop error = %v, want %v", err, stopErr) }
}
