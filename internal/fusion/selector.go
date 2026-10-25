// Package fusion contains policy primitives for assigning new flows across
// independent network paths. It does not combine bytes from multiple paths
// into one TCP/UDP stream; that requires a separate data-plane implementation.
package fusion

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

var (
	ErrNoEligiblePath = errors.New("no eligible fusion path")
	ErrInvalidPaths   = errors.New("invalid fusion path set")
)

// Path is a snapshot supplied by the owner of the actual connections.
// MaxConcurrent <= 0 means unlimited. Weight must be positive.
// Loss is a ratio in [0,1], and RTT must be non-negative.
type Path struct {
	ID            string
	Healthy       bool
	RTT           time.Duration
	Loss          float64
	Weight        int
	ActiveFlows   int
	MaxConcurrent int
}

// Select chooses one path for a new flow using weighted least-cost scoring.
// It is deterministic for identical inputs. The caller owns live counters and
// must update ActiveFlows atomically with connection allocation.
func Select(paths []Path) (Path, error) {
	if len(paths) == 0 {
		return Path{}, ErrNoEligiblePath
	}

	seen := make(map[string]struct{}, len(paths))
	eligible := make([]Path, 0, len(paths))
	for _, path := range paths {
		if path.ID == "" || path.Weight < 1 || path.RTT < 0 ||
			math.IsNaN(path.Loss) || math.IsInf(path.Loss, 0) ||
			path.Loss < 0 || path.Loss > 1 || path.ActiveFlows < 0 ||
			path.MaxConcurrent < 0 {
			return Path{}, fmt.Errorf("%w: malformed path", ErrInvalidPaths)
		}
		if _, exists := seen[path.ID]; exists {
			return Path{}, fmt.Errorf("%w: duplicate path ID %q", ErrInvalidPaths, path.ID)
		}
		seen[path.ID] = struct{}{}
		if !path.Healthy {
			continue
		}
		if path.MaxConcurrent > 0 && path.ActiveFlows >= path.MaxConcurrent {
			continue
		}
		eligible = append(eligible, path)
	}
	if len(eligible) == 0 {
		return Path{}, ErrNoEligiblePath
	}

	score := func(path Path) float64 {
		// Convert before adding to avoid integer overflow at extreme counters.
		// A 1 ms floor prevents a zero-latency sample from dominating forever.
		qualityCost := 1 + float64(path.RTT)/float64(time.Millisecond) + 1000*path.Loss
		return qualityCost * (float64(path.ActiveFlows) + 1) / float64(path.Weight)
	}
	sort.Slice(eligible, func(i, j int) bool {
		a, b := score(eligible[i]), score(eligible[j])
		if a != b {
			return a < b
		}
		if eligible[i].ActiveFlows != eligible[j].ActiveFlows {
			return eligible[i].ActiveFlows < eligible[j].ActiveFlows
		}
		if eligible[i].RTT != eligible[j].RTT {
			return eligible[i].RTT < eligible[j].RTT
		}
		return eligible[i].ID < eligible[j].ID
	})
	return eligible[0], nil
}
