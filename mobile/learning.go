package mobile

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AMINOPY/pg-core/internal/learning"
)

// LearningStore is a JSON-only gomobile facade for bounded connector outcome
// evidence. It records verified outcomes, never connector secrets or traffic.
type LearningStore struct {
	store *learning.Store
}

// NewLearningStore creates an isolated in-memory store. The host may persist
// ExportJSON output in its platform-protected storage and restore it on startup.
func NewLearningStore() *LearningStore {
	return &LearningStore{store: learning.NewStore()}
}

// ObserveJSON accepts one strict JSON observation after the host independently
// verifies the outcome. Failed observations must use latency_ms=-1.
func (s *LearningStore) ObserveJSON(raw string) error {
	if s == nil || s.store == nil || strings.TrimSpace(raw) == "" || len(raw) > 4096 {
		return errInvalidInput
	}
	var observation learning.Observation
	if err := decodeStrict([]byte(raw), &observation); err != nil {
		return fmt.Errorf("%w: learning observation", errInvalidInput)
	}
	if err := s.store.Observe(observation); err != nil {
		return fmt.Errorf("%w: learning observation", errInvalidInput)
	}
	return nil
}

// SnapshotJSON returns records in deterministic connector/version order.
func (s *LearningStore) SnapshotJSON() (string, error) {
	if s == nil || s.store == nil {
		return "", errInvalidInput
	}
	data, err := json.Marshal(s.store.Snapshot())
	if err != nil { return "", err }
	return string(data), nil
}

// ExportJSON returns a versioned backup without credentials or profile data.
func (s *LearningStore) ExportJSON() (string, error) {
	if s == nil || s.store == nil { return "", errInvalidInput }
	data, err := s.store.Export()
	if err != nil { return "", err }
	return string(data), nil
}

// ImportJSON atomically restores a validated backup.
func (s *LearningStore) ImportJSON(raw string) error {
	if s == nil || s.store == nil || len(raw) == 0 || len(raw) > 1<<20 {
		return errInvalidInput
	}
	if err := s.store.Import([]byte(raw)); err != nil {
		return fmt.Errorf("%w: learning snapshot", errInvalidInput)
	}
	return nil
}
