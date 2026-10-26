// Package policy contains deterministic, engine-agnostic selection logic.
package policy

import (
	"errors"
	"math"
	"sort"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
)

var ErrNoHealthyCandidate = errors.New("no eligible engine candidate")

type Candidate struct {
	ID      string
	Status  engine.Status
	Latency time.Duration

	// MetricsValid marks optional measurements as available and independently
	// verified by the caller. PacketLoss and SuccessRate are fractions in [0,1];
	// ThroughputMbps is decimal megabits per second. Missing metrics never
	// masquerade as zero-loss or 100% success.
	MetricsValid   bool
	Jitter         time.Duration
	PacketLoss     float64
	SuccessRate    float64
	ThroughputMbps float64
}

// Select chooses a healthy candidate before any degraded candidate. When
// comparable verified metrics exist, it prefers higher success rate, lower
// packet loss, lower latency, lower jitter, and then higher throughput. When
// metrics are unavailable, it preserves the latency-based ordering.
// ID provides a deterministic final tie-breaker; input order is never used.
func Select(candidates []Candidate) (Candidate, error) {
	eligible := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		if !validCandidate(c) {
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
		a, b := eligible[i], eligible[j]
		healthyA := a.Status == engine.StatusHealthy
		healthyB := b.Status == engine.StatusHealthy
		if healthyA != healthyB {
			return healthyA
		}
		if a.MetricsValid != b.MetricsValid {
			return a.MetricsValid
		}
		if a.MetricsValid && b.MetricsValid {
			if a.SuccessRate != b.SuccessRate {
				return a.SuccessRate > b.SuccessRate
			}
			if a.PacketLoss != b.PacketLoss {
				return a.PacketLoss < b.PacketLoss
			}
			if a.Latency != b.Latency {
				return a.Latency < b.Latency
			}
			if a.Jitter != b.Jitter {
				return a.Jitter < b.Jitter
			}
			if a.ThroughputMbps != b.ThroughputMbps {
				return a.ThroughputMbps > b.ThroughputMbps
			}
		} else if a.Latency != b.Latency {
			return a.Latency < b.Latency
		}
		return a.ID < b.ID
	})
	return eligible[0], nil
}

func validCandidate(c Candidate) bool {
	if c.ID == "" || c.Latency < 0 {
		return false
	}
	if !c.MetricsValid {
		return true
	}
	return c.Jitter >= 0 &&
		!math.IsNaN(c.PacketLoss) && !math.IsInf(c.PacketLoss, 0) &&
		c.PacketLoss >= 0 && c.PacketLoss <= 1 &&
		!math.IsNaN(c.SuccessRate) && !math.IsInf(c.SuccessRate, 0) &&
		c.SuccessRate >= 0 && c.SuccessRate <= 1 &&
		!math.IsNaN(c.ThroughputMbps) && !math.IsInf(c.ThroughputMbps, 0) &&
		c.ThroughputMbps >= 0
}
