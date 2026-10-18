// Package diagnostics provides bounded, consent-based connectivity checks.
// It intentionally does not perform IP-range scanning or traffic impersonation.
package diagnostics

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Result struct {
	Domain       string        `json:"domain"`
	DNSOK        bool          `json:"dns_ok"`
	DNSLatency   time.Duration `json:"dns_latency"`
	HTTPSOK      bool          `json:"https_ok"`
	HTTPSLatency time.Duration `json:"https_latency"`
	HTTPStatus   int           `json:"http_status,omitempty"`
	Error        string        `json:"error,omitempty"`
}

const maxRawDomainBytes = 512

type Checker struct {
	Timeout time.Duration
	Workers int
	Client  *http.Client
}

func (c Checker) timeout() time.Duration {
	if c.Timeout <= 0 {
		return 5 * time.Second
	}
	return c.Timeout
}

func (c Checker) workers() int {
	if c.Workers <= 0 {
		return 4
	}
	if c.Workers > 32 {
		return 32
	}
	return c.Workers
}

// NormalizeDomain accepts a hostname only, not a URL, path, or port.
func NormalizeDomain(raw string) (string, error) {
	// Bound raw input before lowercasing/trimming, which allocate new strings.
	if len(raw) > maxRawDomainBytes {
		return "", errors.New("hostname input is too long")
	}
	host := strings.TrimSuffix(strings.TrimSpace(strings.ToLower(raw)), ".")
	host = strings.TrimSpace(host)
	if host == "" || strings.Contains(host, "://") || strings.ContainsAny(host, "/:@?# ") {
		return "", errors.New("expected a hostname without scheme, port, or path")
	}
	if net.ParseIP(host) != nil {
		return host, nil
	}
	if len(host) > 253 {
		return "", errors.New("hostname is too long")
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("invalid hostname label %q", label)
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' {
				return "", fmt.Errorf("invalid character in hostname label %q", label)
			}
		}
	}
	return host, nil
}

func (c Checker) Check(ctx context.Context, raw string) Result {
	result := Result{Domain: strings.TrimSpace(raw)}
	host, err := NormalizeDomain(raw)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Domain = host
	ctx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()

	start := time.Now()
	ips, err := net.DefaultResolver.LookupHost(ctx, host)
	result.DNSLatency = time.Since(start)
	if err != nil {
		result.Error = "DNS: " + err.Error()
		return result
	}
	if len(ips) == 0 {
		result.Error = "DNS: no addresses returned"
		return result
	}
	result.DNSOK = true

	client := c.Client
	if client == nil {
		client = &http.Client{
			Timeout: c.timeout(),
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		}
	}
	u := url.URL{Scheme: "https", Host: host, Path: "/"}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u.String(), nil)
	if err != nil {
		result.Error = "HTTPS: " + err.Error()
		return result
	}
	start = time.Now()
	resp, err := client.Do(req)
	result.HTTPSLatency = time.Since(start)
	if err != nil {
		result.Error = "HTTPS: " + err.Error()
		return result
	}
	defer resp.Body.Close()
	result.HTTPStatus = resp.StatusCode
	result.HTTPSOK = resp.StatusCode >= 200 && resp.StatusCode < 500
	if !result.HTTPSOK {
		result.Error = fmt.Sprintf("HTTPS: unexpected status %d", resp.StatusCode)
	}
	return result
}

// CheckMany preserves input order and limits concurrent requests.
func (c Checker) CheckMany(ctx context.Context, domains []string) []Result {
	out := make([]Result, len(domains))
	if len(domains) == 0 {
		return out
	}
	type job struct {
		index  int
		domain string
	}
	jobs := make(chan job)
	var wg sync.WaitGroup
	workers := c.workers()
	if workers > len(domains) {
		workers = len(domains)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				out[j.index] = c.Check(ctx, j.domain)
			}
		}()
	}
	for i, domain := range domains {
		jobs <- job{index: i, domain: domain}
	}
	close(jobs)
	wg.Wait()
	return out
}
