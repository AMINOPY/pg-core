// Package diagnostics provides bounded, consent-based connectivity checks.
// It intentionally does not perform IP-range scanning or traffic impersonation.
package diagnostics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBenchmarkTimeout = 10 * time.Second
	defaultBenchmarkBytes   = int64(1 << 20)
	maxBenchmarkBytes       = int64(8 << 20)
)

// BenchmarkResult records a bounded HTTPS transfer sample. Timings are
// observational: connection reuse, caches, server load, and the chosen endpoint
// can affect results. Throughput is payload throughput, not a line-speed claim.
type BenchmarkResult struct {
	URL                 string        `json:"url"`
	DNSLatency          time.Duration `json:"dns_latency"`
	ConnectLatency      time.Duration `json:"connect_latency"`
	TLSHandshakeLatency time.Duration `json:"tls_handshake_latency"`
	FirstByteLatency    time.Duration `json:"first_byte_latency"`
	TotalLatency        time.Duration `json:"total_latency"`
	BytesRead           int64         `json:"bytes_read"`
	ThroughputMbps      float64       `json:"throughput_mbps"`
	HTTPStatus          int           `json:"http_status,omitempty"`
	Error               string        `json:"error,omitempty"`
}

// Benchmarker performs one bounded HTTPS GET against an explicitly supplied
// endpoint. It rejects non-HTTPS URLs, disables redirects, verifies TLS using
// the standard library defaults, and caps both duration and bytes read.
type Benchmarker struct {
	Timeout  time.Duration
	MaxBytes int64
	Client   *http.Client
}

func (b Benchmarker) timeout() time.Duration {
	if b.Timeout <= 0 || b.Timeout > time.Minute {
		return defaultBenchmarkTimeout
	}
	return b.Timeout
}

func (b Benchmarker) maxBytes() int64 {
	if b.MaxBytes <= 0 {
		return defaultBenchmarkBytes
	}
	if b.MaxBytes > maxBenchmarkBytes {
		return maxBenchmarkBytes
	}
	return b.MaxBytes
}

// Benchmark performs a single sample. Callers should compare multiple samples
// and endpoints before changing network configuration based on these results.
func (b Benchmarker) Benchmark(parent context.Context, rawURL string) BenchmarkResult {
	result := BenchmarkResult{URL: strings.TrimSpace(rawURL)}
	if parent == nil {
		result.Error = "context is required"
		return result
	}
	u, err := url.Parse(result.URL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		result.Error = "expected an HTTPS URL without userinfo or fragment"
		return result
	}
	ctx, cancel := context.WithTimeout(parent, b.timeout())
	defer cancel()
	started := time.Now()
	var dnsStart, connectStart, tlsStart time.Time
	trace := &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone: func(httptrace.DNSDoneInfo) {
			if !dnsStart.IsZero() {
				result.DNSLatency = time.Since(dnsStart)
			}
		},
		ConnectStart: func(_, _ string) { connectStart = time.Now() },
		ConnectDone: func(_, _ string, _ error) {
			if !connectStart.IsZero() {
				result.ConnectLatency = time.Since(connectStart)
			}
		},
		TLSHandshakeStart: func() { tlsStart = time.Now() },
		TLSHandshakeDone: func(httptrace.TLSHandshakeDoneInfo, error) {
			if !tlsStart.IsZero() {
				result.TLSHandshakeLatency = time.Since(tlsStart)
			}
		},
		GotFirstResponseByte: func() {
			if result.FirstByteLatency == 0 {
				result.FirstByteLatency = time.Since(started)
			}
		},
	}
	ctx = httptrace.WithClientTrace(ctx, trace)
	client := b.Client
	if client == nil {
		client = &http.Client{}
	}
	clientCopy := *client
	clientCopy.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		result.Error = "create request: " + err.Error()
		return result
	}
	resp, err := clientCopy.Do(req)
	if err != nil {
		result.TotalLatency = time.Since(started)
		if ctx.Err() != nil {
			result.Error = "request deadline exceeded or cancelled"
		} else {
			result.Error = "HTTPS request failed"
		}
		return result
	}
	defer resp.Body.Close()
	result.HTTPStatus = resp.StatusCode
	n, readErr := io.Copy(io.Discard, io.LimitReader(resp.Body, b.maxBytes()))
	result.BytesRead = n
	result.TotalLatency = time.Since(started)
	if result.TotalLatency > 0 {
		result.ThroughputMbps = float64(n*8) / result.TotalLatency.Seconds() / 1_000_000
	}
	if readErr != nil {
		result.Error = "read response body failed"
		return result
	}
	if ctx.Err() != nil {
		result.Error = "request deadline exceeded or cancelled"
		return result
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Error = fmt.Sprintf("unexpected HTTP status %d", resp.StatusCode)
	}
	return result
}
