// Package health provides bounded, consent-based HTTP health probes for
// explicitly configured endpoints. It never discovers or scans endpoints.
package health

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
	"github.com/AMINOPY/pg-core/internal/policy"
)

const (
	DefaultTimeout = 4 * time.Second
	MaxTargets     = 128
	MaxConcurrency = 16
)

var ErrInvalidProbeConfig = errors.New("invalid health probe configuration")

type Target struct {
	ID             string
	SubscriptionID string
	URL            string
}

type ProbeResult struct {
	Candidate policy.ServerCandidate
	CheckedAt time.Time
	ErrorCode string
}

type Prober struct {
	client      *http.Client
	timeout     time.Duration
	concurrency int
	now         func() time.Time
}

func NewProber(timeout time.Duration, concurrency int) (*Prober, error) {
	if timeout <= 0 || concurrency < 1 || concurrency > MaxConcurrency {
		return nil, ErrInvalidProbeConfig
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		ExpectContinueTimeout: time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &Prober{client: client, timeout: timeout, concurrency: concurrency, now: time.Now}, nil
}

// Probe runs bounded concurrent probes against explicitly supplied HTTPS URLs.
// A negative latency sentinel is emitted on failure, never treated as zero.
func (p *Prober) Probe(ctx context.Context, targets []Target) ([]ProbeResult, error) {
	if p == nil || p.client == nil {
		return nil, ErrInvalidProbeConfig
	}
	if len(targets) == 0 || len(targets) > MaxTargets {
		return nil, fmt.Errorf("%w: target count must be between 1 and %d", ErrInvalidProbeConfig, MaxTargets)
	}
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if strings.TrimSpace(target.ID) == "" {
			return nil, fmt.Errorf("%w: target ID is required", ErrInvalidProbeConfig)
		}
		if _, ok := seen[target.ID]; ok {
			return nil, fmt.Errorf("%w: duplicate target ID", ErrInvalidProbeConfig)
		}
		seen[target.ID] = struct{}{}
		if err := validateURL(target.URL); err != nil {
			return nil, fmt.Errorf("%w: target %q URL must be absolute HTTPS", ErrInvalidProbeConfig, target.ID)
		}
	}
	results := make([]ProbeResult, len(targets))
	sem := make(chan struct{}, p.concurrency)
	var wg sync.WaitGroup
	for i, target := range targets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		wg.Add(1)
		go func(i int, target Target) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results[i] = failedResult(target, p.now(), "cancelled")
				return
			}
			defer func() { <-sem }()
			results[i] = p.probeOne(ctx, target)
		}(i, target)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Candidate.ID < results[j].Candidate.ID
	})
	return results, nil
}

func validateURL(raw string) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil || u == nil || !strings.EqualFold(u.Scheme, "https") || u.Hostname() == "" || u.User != nil {
		return ErrInvalidProbeConfig
	}
	return nil
}

func (p *Prober) probeOne(parent context.Context, target Target) ProbeResult {
	start := p.now()
	ctx, cancel := context.WithTimeout(parent, p.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
	if err != nil {
		return failedResult(target, p.now(), "request_invalid")
	}
	req.Header.Set("User-Agent", "Chimera-Core-HealthCheck/1.0")
	resp, err := p.client.Do(req)
	checked := p.now()
	if err != nil {
		return failedResult(target, checked, "probe_failed")
	}
	// Do not read response bodies; closing is enough to release resources.
	_ = resp.Body.Close()
	latency := checked.Sub(start)
	if latency < 0 {
		return failedResult(target, checked, "invalid_latency")
	}
	status := engine.StatusHealthy
	errorCode := ""
	if resp.StatusCode >= http.StatusInternalServerError {
		status, errorCode = engine.StatusFailed, "http_server_error"
	}
	return ProbeResult{
		Candidate: policy.ServerCandidate{
			ID: target.ID, SubscriptionID: target.SubscriptionID,
			Status: status, Latency: latency,
		},
		CheckedAt: checked, ErrorCode: errorCode,
	}
}

func failedResult(target Target, checked time.Time, code string) ProbeResult {
	return ProbeResult{
		Candidate: policy.ServerCandidate{
			ID: target.ID, SubscriptionID: target.SubscriptionID,
			Status: engine.StatusFailed, Latency: -1,
		},
		CheckedAt: checked, ErrorCode: code,
	}
}
