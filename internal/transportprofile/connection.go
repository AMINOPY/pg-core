package transportprofile

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ConnectionMode string

const (
	ModeAuto        ConnectionMode = "auto"
	ModeDirect      ConnectionMode = "direct"
	ModeCDNFronting ConnectionMode = "cdn_fronting"
)

var ErrInvalidConnectionConfig = errors.New("invalid connection config")

const (
	MaxCDNIPs          = 32
	MaxProbeConcurrency = 8
	defaultProbeTimeout = 4 * time.Second
)

// ConnectionConfig describes connection selection for an explicitly configured
// HTTPS service. Endpoint supplies the HTTP authority/origin hostname;
// ServerName supplies the TLS SNI name. A mismatch is accepted only in
// cdn_fronting mode and only with explicitly listed CDN IPs. The CDN operator
// must permit the request and route it to the configured origin.
type ConnectionConfig struct {
	Mode       ConnectionMode `json:"mode"`
	Endpoint   string         `json:"endpoint"`
	ServerName string         `json:"server_name"`
	CDNIPs     []string       `json:"cdn_ips,omitempty"`
	TimeoutMS  int            `json:"timeout_ms,omitempty"`
}

func (c ConnectionConfig) Validate() error {
	if c.Mode != ModeAuto && c.Mode != ModeDirect && c.Mode != ModeCDNFronting {
		return fmt.Errorf("%w: unsupported mode %q", ErrInvalidConnectionConfig, c.Mode)
	}
	u, err := url.Parse(c.Endpoint)
	if err != nil || u == nil || u.Scheme != "https" || u.Hostname() == "" ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		(u.Path != "" && u.Path != "/") {
		return fmt.Errorf("%w: endpoint must be an HTTPS origin without credentials, query, or fragment", ErrInvalidConnectionConfig)
	}
	if port := u.Port(); port != "" && port != "443" {
		return fmt.Errorf("%w: endpoint must use port 443", ErrInvalidConnectionConfig)
	}
	endpointHost := normalizeHost(u.Hostname())
	serverName := normalizeHost(c.ServerName)
	if !validDNSName(endpointHost) || !validDNSName(serverName) {
		return fmt.Errorf("%w: endpoint and server_name must be DNS hostnames", ErrInvalidConnectionConfig)
	}
	if c.TimeoutMS != 0 && (c.TimeoutMS < 250 || c.TimeoutMS > 30000) {
		return fmt.Errorf("%w: timeout_ms must be 250..30000", ErrInvalidConnectionConfig)
	}
	if len(c.CDNIPs) > MaxCDNIPs {
		return fmt.Errorf("%w: at most %d CDN IPs are allowed", ErrInvalidConnectionConfig, MaxCDNIPs)
	}
	seen := make(map[string]struct{}, len(c.CDNIPs))
	for _, raw := range c.CDNIPs {
		ip := net.ParseIP(strings.TrimSpace(raw))
		if ip == nil {
			return fmt.Errorf("%w: invalid CDN IP %q", ErrInvalidConnectionConfig, raw)
		}
		canonical := ip.String()
		if _, ok := seen[canonical]; ok {
			return fmt.Errorf("%w: duplicate CDN IP %q", ErrInvalidConnectionConfig, canonical)
		}
		seen[canonical] = struct{}{}
	}
	switch c.Mode {
	case ModeDirect:
		if endpointHost != serverName {
			return fmt.Errorf("%w: direct mode requires server_name to match endpoint hostname", ErrInvalidConnectionConfig)
		}
		if len(c.CDNIPs) != 0 {
			return fmt.Errorf("%w: direct mode does not use cdn_ips", ErrInvalidConnectionConfig)
		}
	case ModeCDNFronting:
		if len(c.CDNIPs) == 0 {
			return fmt.Errorf("%w: cdn_fronting mode requires explicit cdn_ips", ErrInvalidConnectionConfig)
		}
	case ModeAuto:
		if endpointHost != serverName && len(c.CDNIPs) == 0 {
			return fmt.Errorf("%w: a differing server_name requires explicit cdn_ips", ErrInvalidConnectionConfig)
		}
	}
	return nil
}

// ConnectionAttempt is one ordered candidate produced by PlanConnections.
// Address is a hostname for direct mode and a literal IP for CDN-fronting mode.
type ConnectionAttempt struct {
	Mode       ConnectionMode `json:"mode"`
	Address    string         `json:"address"`
	ServerName string         `json:"server_name"`
	HTTPHost   string         `json:"http_host"`
}

// PlanConnections returns deterministic candidates for the selected mode.
// Auto tries direct first, then each explicitly configured CDN IP. The caller
// remains responsible for attempting candidates and checking end-to-end traffic.
func PlanConnections(cfg ConnectionConfig) ([]ConnectionAttempt, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	u, _ := url.Parse(cfg.Endpoint)
	host := normalizeHost(u.Hostname())
	direct := ConnectionAttempt{Mode: ModeDirect, Address: host, ServerName: host, HTTPHost: host}
	fronting := func(ip string) ConnectionAttempt {
		return ConnectionAttempt{Mode: ModeCDNFronting, Address: ip, ServerName: normalizeHost(cfg.ServerName), HTTPHost: host}
	}
	switch cfg.Mode {
	case ModeDirect:
		return []ConnectionAttempt{direct}, nil
	case ModeCDNFronting:
		out := make([]ConnectionAttempt, 0, len(cfg.CDNIPs))
		for _, ip := range cfg.CDNIPs {
			out = append(out, fronting(net.ParseIP(strings.TrimSpace(ip)).String()))
		}
		return out, nil
	case ModeAuto:
		out := []ConnectionAttempt{direct}
		for _, ip := range cfg.CDNIPs {
			out = append(out, fronting(net.ParseIP(strings.TrimSpace(ip)).String()))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%w: unsupported mode %q", ErrInvalidConnectionConfig, cfg.Mode)
	}
}

// ProbeResult reports whether an explicitly supplied IP accepts a verified TLS
// handshake for ServerName. Reachable means TLS succeeded, not that a tunnel,
// HTTP route, or CDN origin mapping works.
type ProbeResult struct {
	IP        string        `json:"ip"`
	Reachable bool          `json:"reachable"`
	Latency   time.Duration `json:"latency"`
	Error     string        `json:"error,omitempty"`
}

// ProbeCDNIPs checks only the explicitly configured IP list. It never scans
// ranges or discovers additional hosts. Certificate verification remains on.
func ProbeCDNIPs(ctx context.Context, cfg ConnectionConfig) []ProbeResult {
	if err := cfg.Validate(); err != nil || len(cfg.CDNIPs) == 0 {
		return nil
	}
	timeout := defaultProbeTimeout
	if cfg.TimeoutMS != 0 {
		timeout = time.Duration(cfg.TimeoutMS) * time.Millisecond
	}
	results := make([]ProbeResult, len(cfg.CDNIPs))
	workers := len(cfg.CDNIPs)
	if workers > MaxProbeConcurrency {
		workers = MaxProbeConcurrency
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i] = probeOne(ctx, cfg.CDNIPs[i], cfg.ServerName, timeout)
			}
		}()
	}
	for i := range cfg.CDNIPs {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return results
}

func probeOne(ctx context.Context, rawIP, serverName string, timeout time.Duration) ProbeResult {
	ip := net.ParseIP(strings.TrimSpace(rawIP))
	result := ProbeResult{IP: ip.String()}
	start := time.Now()
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(probeCtx, "tcp", net.JoinHostPort(result.IP, strconv.Itoa(443)))
	if err != nil {
		result.Error = "tcp connection failed"
		result.Latency = time.Since(start)
		return result
	}
	defer conn.Close()
	tlsConn := tls.Client(conn, &tls.Config{ServerName: normalizeHost(serverName), MinVersion: tls.VersionTLS12})
	if err := tlsConn.HandshakeContext(probeCtx); err != nil {
		result.Error = "tls handshake or certificate verification failed"
		result.Latency = time.Since(start)
		return result
	}
	result.Reachable = true
	result.Latency = time.Since(start)
	return result
}
