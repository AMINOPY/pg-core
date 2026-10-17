// Package policy contains deterministic, engine-agnostic selection logic.
package policy

import (
	"errors"
	"sort"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
)

var ErrNoHealthyCandidate = errors.New("no eligible engine candidate")

type Candidate struct {
	ID      string
	Status  engine.Status
	Latency time.Duration
}

// Select chooses a healthy candidate with the lowest non-negative latency.
// Degraded candidates are considered only when no healthy candidate exists.
// Input order is not used as a tie-breaker; ID provides deterministic ordering.
func Select(candidates []Candidate) (Candidate, error) {
	eligible := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		if c.ID == "" || c.Latency < 0 {
			continue
		}
		if c.Status == engine.StatusHealthy || c.Status == engine.StatusDegraded {
			eligible = append(eligible, c)
		}
	}
	if len(eligible) == 0 {
		return Candidate{}, ErrNoHealthyCandidate
	}

	sort.Slice(eligible, func(i, j int) bool {
		hi := eligible[i].Status == engine.StatusHealthy
		hj := eligible[j].Status == engine.StatusHealthy
		if hi != hj {
			return hi
		}
		if eligible[i].Latency != eligible[j].Latency {
			return eligible[i].Latency < eligible[j].Latency
		}
		return eligible[i].ID < eligible[j].ID
	})
	return eligible[0], nil
}
