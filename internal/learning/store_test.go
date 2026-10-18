package learning

import (
	"errors"
	"testing"
	"time"
)

func TestObserveAndExportImport(t *testing.T) {
	s := NewStore()
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	s.now = func() time.Time { return now }
	if err := s.Observe(Observation{ConnectorID: "alpha", Version: "1", Success: true, LatencyMS: 200}); err != nil { t.Fatal(err) }
	if err := s.Observe(Observation{ConnectorID: "alpha", Version: "1", Success: false, LatencyMS: -1}); err != nil { t.Fatal(err) }
	got := s.Snapshot()
	if len(got) != 1 || got[0].Samples != 2 || got[0].Successes != 1 || got[0].Failures != 1 || got[0].ConsecutiveFailures != 1 { t.Fatalf("unexpected record: %+v", got) }
	data, err := s.Export(); if err != nil { t.Fatal(err) }
	other := NewStore(); if err := other.Import(data); err != nil { t.Fatal(err) }
	if len(other.Snapshot()) != 1 || other.Snapshot()[0] != got[0] { t.Fatal("round-trip mismatch") }
}

func TestRejectsInvalidAndImportIsAtomic(t *testing.T) {
	s := NewStore()
	if err := s.Observe(Observation{ConnectorID: " ", Version: "1", Success: true, LatencyMS: 1}); !errors.Is(err, ErrInvalidObservation) { t.Fatalf("got %v", err) }
	_ = s.Observe(Observation{ConnectorID: "keep", Version: "1", Success: true, LatencyMS: 10})
	if err := s.Import([]byte(`{"schema_version":1,"records":[{"connector_id":"x","version":"1","samples":2,"successes":1,"failures":0,"mean_latency_ms":1,"last_observed":"2026-01-01T00:00:00Z","consecutive_failures":0}]}`)); err == nil { t.Fatal("expected invalid counters") }
	if len(s.Snapshot()) != 1 || s.Snapshot()[0].ConnectorID != "keep" { t.Fatal("invalid import mutated store") }
	if err := s.Import([]byte(`{"schema_version":1,"records":[],"unexpected":true}`)); err == nil { t.Fatal("expected unknown field rejection") }
}

func TestScoreIsBoundedAndPenalizesFailures(t *testing.T) {
	good := Record{Samples: 10, Successes: 10, MeanLatencyMS: 50}
	bad := Record{Samples: 10, Successes: 5, Failures: 5, ConsecutiveFailures: 3, MeanLatencyMS: 50}
	if Score(good) <= Score(bad) || Score(good) > 100 || Score(bad) < 0 { t.Fatalf("scores good=%v bad=%v", Score(good), Score(bad)) }
}
