package fusion

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	maxProbeRouteCount = 256
	maxProbeWorkers    = 32
	maxProbeTimeout    = time.Minute
)

// RouteProbe checks whether a route can establish the kind of connection the
// host intends to use. Implementations must honor context cancellation.
type RouteProbe func(context.Context, Route) (time.Duration, error)

// RefreshRoutes probes a bounded set of routes concurrently, then atomically
// updates the router's path snapshots. A failed probe marks only that route
// unhealthy; the returned failure map lets callers explain why. Parent
// cancellation or invalid input leaves the router's previous snapshot intact.
//
// This function measures only what the supplied probe actually verifies. For
// example, a TCP connect probe is not proof that DNS, HTTPS, or the whole VPN
// data plane works.
func RefreshRoutes(
	ctx context.Context,
	router *Router,
	routes []Route,
	probe RouteProbe,
	maxConcurrent int,
	timeout time.Duration,
) (map[string]error, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if router == nil {
		return nil, errors.New("fusion router is required")
	}
	if probe == nil {
		return nil, errors.New("route probe is required")
	}
	if maxConcurrent < 1 || maxConcurrent > maxProbeWorkers {
		return nil, fmt.Errorf("probe concurrency must be between 1 and %d", maxProbeWorkers)
	}
	if timeout <= 0 || timeout > maxProbeTimeout {
		return nil, fmt.Errorf("probe timeout must be positive and at most %s", maxProbeTimeout)
	}
	if len(routes) > maxProbeRouteCount {
		return nil, fmt.Errorf("route count exceeds limit of %d", maxProbeRouteCount)
	}
	paths, _, err := normalizeRoutes(routes)
	if err != nil {
		return nil, err
	}
	if _, err := NewAllocator(paths); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	updated := append([]Route(nil), routes...)
	probeErrors := make([]error, len(routes))
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	for i := range updated {
		if err := ctx.Err(); err != nil {
			break
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				probeErrors[i] = ctx.Err()
				return
			}

			probeCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			rtt, probeErr := probe(probeCtx, updated[i])
			if probeErr == nil {
				probeErr = probeCtx.Err()
			}
			if probeErr == nil && rtt < 0 {
				probeErr = errors.New("route probe returned a negative RTT")
			}
			if probeErr != nil {
				updated[i].Path.Healthy = false
				updated[i].Path.RTT = 0
				probeErrors[i] = probeErr
				return
			}
			updated[i].Path.Healthy = true
			updated[i].Path.RTT = rtt
		}(i)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	failures := make(map[string]error)
	for i, err := range probeErrors {
		if err != nil {
			failures[updated[i].Path.ID] = err
		}
	}
	if err := router.Update(updated); err != nil {
		return nil, err
	}
	return failures, nil
}
