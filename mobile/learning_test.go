package mobile

import (
	"strings"
	"testing"
)

func TestMobileLearningStoreRoundTrip(t *testing.T) {
	s := NewLearningStore()
	if err := s.ObserveJSON(`{"connector_id":"direct","version":"1.2","success":true,"latency_ms":120}`); err != nil { t.Fatal(err) }
	if err := s.ObserveJSON(`{"connector_id":"direct","version":"1.2","success":false,"latency_ms":-1}`); err != nil { t.Fatal(err) }
	if err := s.ObserveJSON(`{"connector_id":"direct","version":"1.2","success":true,"latency_ms":80,"extra":true}`); err == nil { t.Fatal("unknown field accepted") }
	snapshot, err := s.SnapshotJSON(); if err != nil { t.Fatal(err) }
	if !strings.Contains(snapshot, `"samples":2`) || !strings.Contains(snapshot, `"consecutive_failures":0`) { t.Fatalf("unexpected snapshot: %s", snapshot) }
	exported, err := s.ExportJSON(); if err != nil { t.Fatal(err) }
	restored := NewLearningStore()
	if err := restored.ImportJSON(exported); err != nil { t.Fatal(err) }
	roundTrip, err := restored.SnapshotJSON(); if err != nil { t.Fatal(err) }
	if snapshot != roundTrip { t.Fatalf("snapshot mismatch: %s != %s", snapshot, roundTrip) }
}

func TestMobileLearningRejectsMalformedInput(t *testing.T) {
	s := NewLearningStore()
	for _, raw := range []string{"", "{", `{"connector_id":"x","version":"1","success":false,"latency_ms":3}`, `{"connector_id":"x","version":"1","success":true,"latency_ms":1,"extra":1}`} {
		if err := s.ObserveJSON(raw); err == nil { t.Fatalf("accepted invalid observation %q", raw) }
	}
	if err := s.ImportJSON(strings.Repeat("x", (1<<20)+1)); err == nil { t.Fatal("oversized import accepted") }
}
