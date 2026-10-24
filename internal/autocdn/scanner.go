// Package autocdn selects a low-latency TLS route from an explicitly supplied
// candidate catalog. Candidate lists must come from an authorized deployment or
// an export produced by the operator's Network Checker workflow; this package
// does not discover or invent public hosts/IP ranges.
package autocdn

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	MaxCandidates   = 1000
	MaxPairProbes   = 1000
	defaultWorkers  = 32
	MaxRankedRoutes = 20
	defaultTimeout  = 1800 * time.Millisecond
)

type Catalog struct {
	SNIs   []string `json:"sni"`
	CDNIPs []string `json:"cdn_ips"`
}

type Route struct {
	SNI       string        `json:"sni"`
	CDNIP     string        `json:"cdn_ip"`
	Latency   time.Duration `json:"latency_ns"`
	CheckedAt time.Time     `json:"checked_at"`
}

type Pair struct {
	SNI   string
	CDNIP string
}

type ProbeFunc func(context.Context, string, string) (time.Duration, error)

type Scanner struct {
	Workers int
	Timeout time.Duration
	Budget  int
}

func (c Catalog) Validate() error {
	if len(c.SNIs) == 0 || len(c.CDNIPs) == 0 {
		return errors.New("auto catalog must contain at least one SNI and one CDN IP")
	}
	if len(c.SNIs) > MaxCandidates {
		return fmt.Errorf("auto catalog has more than %d SNI candidates", MaxCandidates)
	}
	if len(c.CDNIPs) > MaxCandidates {
		return fmt.Errorf("auto catalog has more than %d CDN IP candidates", MaxCandidates)
	}
	seenSNI := make(map[string]bool, len(c.SNIs))
	for _, raw := range c.SNIs {
		host := strings.TrimSpace(raw)
		if host != raw {
			return fmt.Errorf("invalid SNI candidate %q: surrounding whitespace is not allowed", raw)
		}
		if err := validateHostname(host); err != nil {
			return fmt.Errorf("invalid SNI candidate %q: %w", raw, err)
		}
		key := strings.ToLower(strings.TrimSuffix(host, "."))
		if seenSNI[key] {
			return fmt.Errorf("duplicate SNI candidate %q", host)
		}
		seenSNI[key] = true
	}
	seenIP := make(map[netip.Addr]bool, len(c.CDNIPs))
	for _, raw := range c.CDNIPs {
		trimmed := strings.TrimSpace(raw)
		if trimmed != raw {
			return fmt.Errorf("invalid CDN IP candidate %q: surrounding whitespace is not allowed", raw)
		}
		addr, err := netip.ParseAddr(trimmed)
		if err != nil {
			return fmt.Errorf("invalid CDN IP candidate %q", raw)
		}
		addr = addr.Unmap()
		if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsUnspecified() {
			return fmt.Errorf("CDN IP candidate %q must be a public unicast address", raw)
		}
		if seenIP[addr] {
			return fmt.Errorf("duplicate CDN IP candidate %q", raw)
		}
		seenIP[addr] = true
	}
	return nil
}

func validateHostname(host string) error {
	if host == "" || len(host) > 253 {
		return errors.New("hostname is empty or too long")
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return errors.New("SNI must be a hostname, not an IP address")
	}
	for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return errors.New("invalid DNS hostname")
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') &&
				!(r >= '0' && r <= '9') && r != '-' {
				return errors.New("invalid DNS hostname")
			}
		}
	}
	return nil
}

// Select performs a bounded set of TLS probes across the trusted candidate
// catalog. It tests up to 1,000 SNI/IP pairs (rather than the full million-pair
// cross product) and returns the lowest-latency pair with a valid TLS certificate.
func (s Scanner) Select(ctx context.Context, catalog Catalog, probe ProbeFunc) (Route, error) {
	routes, err := s.SelectCandidates(ctx, catalog, probe)
	if err != nil {
		return Route{}, err
	}
	return routes[0], nil
}

// SelectCandidates returns up to twenty healthy TLS pairs, ordered by measured
// latency. The caller must still verify each route with the actual tunnel engine.
func (s Scanner) SelectCandidates(ctx context.Context, catalog Catalog, probe ProbeFunc) ([]Route, error) {
	if err := catalog.Validate(); err != nil {
		return nil, err
	}
	if probe == nil {
		probe = ProbeTLS
	}
	workers := s.Workers
	if workers <= 0 || workers > 64 {
		workers = defaultWorkers
	}
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	budget := s.Budget
	if budget <= 0 || budget > MaxPairProbes {
		budget = MaxPairProbes
	}
	pairs := makePairs(catalog, budget)
	jobs := make(chan Pair)
	results := make(chan Route, len(pairs))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pair := range jobs {
				probeCtx, cancel := context.WithTimeout(ctx, timeout)
				latency, err := probe(probeCtx, pair.SNI, pair.CDNIP)
				cancel()
				if err == nil {
					results <- Route{SNI: pair.SNI, CDNIP: pair.CDNIP, Latency: latency, CheckedAt: time.Now().UTC()}
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, pair := range pairs {
			select {
			case <-ctx.Done():
				return
			case jobs <- pair:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()
	routes := make([]Route, 0, MaxRankedRoutes)
	for route := range results {
		routes = append(routes, route)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return nil, errors.New("no candidate pair passed TLS validation; TLS success alone also does not prove the tunnel's origin routing works")
	}
	Rank(routes)
	if len(routes) > MaxRankedRoutes {
		routes = routes[:MaxRankedRoutes]
	}
	return routes, nil
}

func makePairs(c Catalog, budget int) []Pair {
	maxPairs := len(c.SNIs) * len(c.CDNIPs)
	if budget > maxPairs {
		budget = maxPairs
	}
	pairs := make([]Pair, 0, budget)
	seen := make(map[string]bool, budget)
	// The first sweep includes every SNI and every IP when both lists contain
	// at most 1000 entries. Further sweeps vary pairings without scanning the
	// entire Cartesian product.
	for round := 0; len(pairs) < budget; round++ {
		before := len(pairs)
		for i, sni := range c.SNIs {
			j := (i*7919 + round*104729) % len(c.CDNIPs)
			ip := c.CDNIPs[j]
			key := strings.ToLower(sni) + "\x00" + ip
			if seen[key] {
				continue
			}
			seen[key] = true
			pairs = append(pairs, Pair{SNI: sni, CDNIP: ip})
			if len(pairs) == budget {
				break
			}
		}
		if len(pairs) == before {
			break
		}
	}
	return pairs
}

// ProbeTLS checks a public CDN IP with the candidate SNI and normal certificate
// verification. It does not disable TLS verification or claim that the origin
// tunnel is healthy; the selected route must pass the engine's own health check.
func ProbeTLS(ctx context.Context, sni, ip string) (time.Duration, error) {
	start := time.Now()
	dialer := &net.Dialer{}
	raw, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, "443"))
	if err != nil {
		return 0, err
	}
	defer raw.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(deadline)
	}
	conn := tls.Client(raw, &tls.Config{ServerName: sni, MinVersion: tls.VersionTLS12})
	if err := conn.HandshakeContext(ctx); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

// Rank sorts already-validated route results by latency, with stable SNI/IP
// tie-breaking for reproducible displays and tests.
func Rank(routes []Route) {
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].Latency != routes[j].Latency {
			return routes[i].Latency < routes[j].Latency
		}
		if routes[i].SNI != routes[j].SNI {
			return routes[i].SNI < routes[j].SNI
		}
		return routes[i].CDNIP < routes[j].CDNIP
	})
}
