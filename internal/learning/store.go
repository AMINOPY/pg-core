// Package learning keeps bounded, non-secret compatibility evidence for connectors.
// Observations can rank already-authorized connectors; they never grant permissions.
package learning

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxIDLength    = 256
	maxRecords     = 2048
	maxImportBytes = 1 << 20
	maxLatencyMS   = int64(7 * 24 * 60 * 60 * 1000)
)

var (
	ErrInvalidObservation = errors.New("invalid connector observation")
	ErrStoreFull          = errors.New("learning store capacity reached")
)

type Observation struct {
	ConnectorID string `json:"connector_id"`
	Version     string `json:"version"`
	Success     bool   `json:"success"`
	LatencyMS   int64  `json:"latency_ms"`
}

type Record struct {
	ConnectorID         string    `json:"connector_id"`
	Version             string    `json:"version"`
	Samples             uint64    `json:"samples"`
	Successes           uint64    `json:"successes"`
	Failures            uint64    `json:"failures"`
	MeanLatencyMS       float64   `json:"mean_latency_ms"`
	LastObserved        time.Time `json:"last_observed"`
	ConsecutiveFailures uint32    `json:"consecutive_failures"`
}

type key struct{ id, version string }

type Store struct {
	mu      sync.RWMutex
	records map[key]Record
	now     func() time.Time
}

func NewStore() *Store {
	return &Store{records: make(map[key]Record), now: time.Now}
}

// Observe records a verified outcome. Latency -1 means unavailable/not measured.
// Failed observations never contribute latency. Call only after the host has
// verified the result; untrusted connector self-reports are not evidence.
func (s *Store) Observe(o Observation) error {
	if s == nil || strings.TrimSpace(o.ConnectorID) == "" || len(o.ConnectorID) > maxIDLength ||
		strings.TrimSpace(o.Version) == "" || len(o.Version) > maxIDLength ||
		o.LatencyMS < -1 || o.LatencyMS > maxLatencyMS || (!o.Success && o.LatencyMS >= 0) {
		return ErrInvalidObservation
	}
	k := key{o.ConnectorID, o.Version}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, exists := s.records[k]
	if !exists && len(s.records) >= maxRecords {
		return ErrStoreFull
	}
	if !exists {
		r = Record{ConnectorID: o.ConnectorID, Version: o.Version}
	}
	if r.Samples == ^uint64(0) || (o.Success && r.Successes == ^uint64(0)) || (!o.Success && r.Failures == ^uint64(0)) {
		return ErrStoreFull
	}
	r.Samples++
	if o.Success {
		r.Successes++
		r.ConsecutiveFailures = 0
		if o.LatencyMS >= 0 {
			if r.Successes == 1 || r.MeanLatencyMS == 0 {
				r.MeanLatencyMS = float64(o.LatencyMS)
			} else {
				r.MeanLatencyMS = 0.25*float64(o.LatencyMS) + 0.75*r.MeanLatencyMS
			}
		}
	} else {
		r.Failures++
		if r.ConsecutiveFailures < ^uint32(0) {
			r.ConsecutiveFailures++
		}
	}
	r.LastObserved = s.now().UTC()
	s.records[k] = r
	return nil
}

// Snapshot returns a stable copy ordered by connector ID and version.
func (s *Store) Snapshot() []Record {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Record, 0, len(s.records))
	for _, r := range s.records {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ConnectorID != out[j].ConnectorID {
			return out[i].ConnectorID < out[j].ConnectorID
		}
		return out[i].Version < out[j].Version
	})
	return out
}

// Score is an evidence-based ranking hint in [0,100]. It is deliberately not
// an authorization decision; callers must separately enforce manifest policy.
// Insufficiently observed connectors receive a conservative score.
func Score(r Record) float64 {
	if r.Samples == 0 || r.Successes > r.Samples || r.Failures > r.Samples ||
		r.Successes != r.Samples-r.Failures || r.MeanLatencyMS < 0 || r.MeanLatencyMS > float64(maxLatencyMS) {
		return 0
	}
	successRate := float64(r.Successes) / float64(r.Samples)
	confidence := math.Min(1, float64(r.Samples)/10)
	latencyFactor := 1.0
	if r.MeanLatencyMS > 0 {
		latencyFactor = 1000 / (1000 + r.MeanLatencyMS)
	}
	penalty := math.Min(0.5, float64(r.ConsecutiveFailures)*0.1)
	score := 100 * (successRate * confidence * latencyFactor) * (1 - penalty)
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

type snapshot struct {
	SchemaVersion int      `json:"schema_version"`
	Records       []Record `json:"records"`
}

// Export serializes sanitized evidence only; no profiles, credentials or raw traffic.
func (s *Store) Export() ([]byte, error) {
	if s == nil {
		return nil, ErrInvalidObservation
	}
	return json.Marshal(snapshot{SchemaVersion: 1, Records: s.Snapshot()})
}

// Import replaces the store only after strict validation. It rejects unknown
// fields, duplicate keys, invalid counters and oversized payloads.
func (s *Store) Import(data []byte) error {
	if s == nil || len(data) == 0 || len(data) > maxImportBytes {
		return ErrInvalidObservation
	}
	var in snapshot
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&in); err != nil {
		return ErrInvalidObservation
	}
	var trailing any
	if err := d.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ErrInvalidObservation
	}
	if in.SchemaVersion != 1 || len(in.Records) > maxRecords {
		return ErrInvalidObservation
	}
	next := make(map[key]Record, len(in.Records))
	for _, r := range in.Records {
		if strings.TrimSpace(r.ConnectorID) == "" || len(r.ConnectorID) > maxIDLength ||
			strings.TrimSpace(r.Version) == "" || len(r.Version) > maxIDLength ||
			r.Samples == 0 || r.Successes > r.Samples || r.Failures > r.Samples || r.Successes != r.Samples-r.Failures ||
			r.MeanLatencyMS < 0 || r.MeanLatencyMS > float64(maxLatencyMS) ||
			uint64(r.ConsecutiveFailures) > r.Failures || r.LastObserved.IsZero() {
			return ErrInvalidObservation
		}
		k := key{r.ConnectorID, r.Version}
		if _, ok := next[k]; ok {
			return ErrInvalidObservation
		}
		next[k] = r
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = next
	return nil
}
