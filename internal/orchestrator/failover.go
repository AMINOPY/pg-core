package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Candidate is one engine configuration in caller-defined preference order.
// Config and Claims are passed to Fleet.Start unchanged; each candidate must
// already be registered with the fleet.
type Candidate struct {
	EngineID string
	Config   []byte
	Claims   []string
}

// StartFirstAvailable tries candidates in preference order and returns the ID
// of the first engine that starts and passes its adapter health check.
//
// This is failover orchestration, not packet-level traffic aggregation: exactly
// one candidate is selected. If startup cleanup leaves resource ownership
// uncertain, the method requires an explicit successful Stop before it tries
// another candidate. It never stops an unrelated engine.
func (f *Fleet) StartFirstAvailable(ctx context.Context, candidates []Candidate) (string, error) {
	if f == nil {
		return "", errors.New("fleet is required")
	}
	if ctx == nil {
		return "", ErrContextRequired
	}
	if len(candidates) == 0 {
		return "", errors.New("at least one engine candidate is required")
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		id := strings.TrimSpace(candidate.EngineID)
		if id == "" {
			return "", errors.New("candidate engine ID must not be empty")
		}
		if id != candidate.EngineID {
			return "", fmt.Errorf("candidate engine ID %q must not contain surrounding whitespace", candidate.EngineID)
		}
		if _, ok := seen[id]; ok {
			return "", fmt.Errorf("duplicate engine candidate %q", id)
		}
		seen[id] = struct{}{}
	}

	failures := make([]error, 0, len(candidates))
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		err := f.Start(ctx, candidate.EngineID, candidate.Config, candidate.Claims...)
		if err == nil {
			return candidate.EngineID, nil
		}
		failures = append(failures, fmt.Errorf("%s: %w", candidate.EngineID, err))

		// A resource conflict is an external ownership conflict, not evidence
		// that this candidate is unhealthy. Do not continue probing engines
		// against a resource another engine still owns.
		if errors.Is(err, ErrResourceConflict) {
			return "", errors.Join(failures...)
		}

		// Start may have failed after partially acquiring TUN, DNS, routes, or
		// listeners. Retry cleanup explicitly; never advance while ownership is
		// uncertain.
		if errors.Is(err, ErrResourceStateUncertain) {
			if stopErr := f.Stop(ctx, candidate.EngineID); stopErr != nil {
				return "", fmt.Errorf("candidate %q failed and cleanup could not be confirmed: %w", candidate.EngineID, errors.Join(err, stopErr))
			}
		}
	}
	return "", fmt.Errorf("no engine candidate started successfully: %w", errors.Join(failures...))
}
