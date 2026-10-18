package mobile

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/AMINOPY/pg-core/internal/learning"
)

// LearningStore is a JSON-only gomobile facade for bounded connector outcome
// evidence. It records verified outcomes, never connector secrets or traffic.
type LearningStore struct {
	store *learning.Store
}

type rankedRecord struct {
	learning.Record
	Score float64 `json:"score"`
}

// NewLearningStore creates an isolated in-memory store. The host may persist
// ExportJSON output in platform-protected storage and restore it on startup.
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

// SnapshotJSON returns sanitized records in stable connector/version order.
func (s *LearningStore) SnapshotJSON() (string, error) {
	if s == nil || s.store == nil {
		return "", errInvalidInput
	}
	data, err := json.Marshal(s.store.Snapshot())
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// RankedJSON returns records sorted by an evidence-based score (highest first).
// Scores are hints only and must never bypass trusted permission policy.
func (s *LearningStore) RankedJSON() (string, error) {
	if s == nil || s.store == nil {
		return "", errInvalidInput
	}
	records := s.store.Snapshot()
	ranked := make([]rankedRecord, 0, len(records))
	for _, record := range records {
		ranked = append(ranked, rankedRecord{Record: record, Score: learning.Score(record)})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		if ranked[i].ConnectorID != ranked[j].ConnectorID {
			return ranked[i].ConnectorID < ranked[j].ConnectorID
		}
		return ranked[i].Version < ranked[j].Version
	})
	data, err := json.Marshal(ranked)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ExportJSON returns a versioned backup without credentials or profile data.
func (s *LearningStore) ExportJSON() (string, error) {
	if s == nil || s.store == nil {
		return "", errInvalidInput
	}
	data, err := s.store.Export()
	if err != nil {
		return "", err
	}
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
