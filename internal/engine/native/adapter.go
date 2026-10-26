// Package native is Chimera's own tunnel engine: no external binary. It speaks
// a small WebSocket-over-TLS protocol to a server (the Cloudflare Worker in
// deploy/, or Server on a VPS) and exposes a local SOCKS5 proxy.
//
// With the CDN/SNI feature (SNI + edge IP list) the client connects to the
// listed edge IPs and announces the SNI. No other configuration is needed
// except the secret key of your own server.
package native

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
	"github.com/AMINOPY/pg-core/internal/fronting"
)

// Profile is the native engine profile (JSON).
type Profile struct {
	// Key is the secret of your server; the tunnel path is "/" + Key.
	Key string `json:"key,omitempty"`
	// Path overrides the tunnel path (must start with "/").
	Path string `json:"path,omitempty"`
	// Server is used only when CDN/SNI is not enabled: "host" or "host:port".
	Server string `json:"server,omitempty"`
	// Host is the HTTP Host header; defaults to the SNI (or Server host).
	Host string `json:"host,omitempty"`
	// Port of the edge (default 443).
	Port int `json:"port,omitempty"`
	// Listen is the local SOCKS5 address; must be loopback. Default 127.0.0.1:1080.
	Listen    string `json:"listen,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
	// HealthURL optionally verifies an HTTP response through the active SOCKS5 tunnel.
	HealthURL string `json:"health_url,omitempty"`
}

type running struct {
	ln        net.Listener
	cancel    context.CancelFunc
	dialer    *Dialer
	healthURL string
}

// Adapter implements engine.Adapter.
type Adapter struct {
	// Fronting is the CDN/SNI feature. nil or incomplete = use Profile.Server.
	Fronting *fronting.Settings
	// Control is passed to every outgoing socket (Android: VpnService.protect).
	Control func(network, address string, c syscall.RawConn) error

	mu  sync.Mutex
	run *running
}

func New() *Adapter { return &Adapter{} }

func (a *Adapter) ID() string { return "native" }

func (a *Adapter) Capabilities() engine.Capabilities {
	// SOCKS5 only: no TUN, DNS or routing; use tun2socks on the platform side.
	return engine.Capabilities{HealthChecks: true}
}

func parseProfile(raw []byte) (Profile, error) {
	var p Profile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return p, errors.New("invalid native profile")
	}
	// A profile is exactly one JSON object. Reject concatenated values and
	// trailing garbage instead of silently accepting a valid prefix.
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return p, errors.New("invalid native profile: trailing data")
	}
	if err := validateHealthURL(p.HealthURL); err != nil {
		return p, err
	}
	if p.Path == "" {
		if p.Key == "" {
			return p, errors.New("profile needs key or path")
		}
		p.Path = "/" + p.Key
	}
	if !strings.HasPrefix(p.Path, "/") || len(p.Path) < 2 {
		return p, errors.New("path must start with / and not be empty")
	}
	if p.Listen == "" {
		p.Listen = "127.0.0.1:1080"
	}
	host, _, err := net.SplitHostPort(p.Listen)
	if err != nil {
		return p, errors.New("listen must be host:port")
	}
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return p, errors.New("listen must be a loopback address")
	}
	if p.Port < 0 || p.Port > 65535 {
		return p, errors.New("invalid port")
	}
	return p, nil
}

func (a *Adapter) dialer(p Profile) (*Dialer, error) {
	d := &Dialer{Host: p.Host, Port: p.Port, Path: p.Path, Control: a.Control}
	if p.TimeoutMS > 0 {
		d.Timeout = time.Duration(p.TimeoutMS) * time.Millisecond
	}
	if a.Fronting != nil && a.Fronting.Enabled() {
		if err := a.Fronting.Validate(); err != nil {
			return nil, err
		}
		ips, _ := fronting.ParseIPs(a.Fronting.EdgeIPs)
		d.SNI = strings.TrimSpace(a.Fronting.SNI)
		d.EdgeIPs = ips
		return d, nil
	}
	if p.Server == "" {
		return nil, errors.New("enable CDN/SNI (SNI + IP list) or set server in the profile")
	}
	host, port := p.Server, ""
	if h, pt, err := net.SplitHostPort(p.Server); err == nil {
		host, port = h, pt
	}
	d.SNI = host
	d.EdgeIPs = []string{host}
	if port != "" && d.Port == 0 {
		var n int
		if _, err := fmt.Sscanf(port, "%d", &n); err != nil || n < 1 || n > 65535 {
			return nil, errors.New("invalid server port")
		}
		d.Port = n
	}
	return d, nil
}

func (a *Adapter) ValidateConfig(ctx context.Context, raw []byte) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	p, err := parseProfile(raw)
	if err != nil {
		return err
	}
	_, err = a.dialer(p)
	return err
}

func (a *Adapter) Start(ctx context.Context, raw []byte) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	p, err := parseProfile(raw)
	if err != nil {
		return err
	}
	d, err := a.dialer(p)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.run != nil {
		return errors.New("native engine is already running")
	}
	ln, err := net.Listen("tcp", p.Listen)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	go serveSOCKS5(runCtx, ln, d)
	a.run = &running{ln: ln, cancel: cancel, dialer: d, healthURL: p.HealthURL}
	return nil
}

// Addr returns the local SOCKS5 address once started.
func (a *Adapter) Addr() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.run == nil {
		return ""
	}
	return a.run.ln.Addr().String()
}

// Health performs a real probe: TLS to an edge plus the WebSocket upgrade.
func (a *Adapter) Health(ctx context.Context) (engine.HealthReport, error) {
	start := time.Now()
	rep := engine.HealthReport{EngineID: "native", CheckedAt: time.Now().UTC()}
	if ctx == nil {
		return rep, errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return rep, err
	}
	a.mu.Lock()
	r := a.run
	a.mu.Unlock()
	if r == nil {
		rep.Status, rep.ErrorCode = engine.StatusStopped, "not_started"
		return rep, nil
	}
	if err := r.dialer.Probe(ctx); err != nil {
		if ctx.Err() != nil {
			return rep, ctx.Err()
		}
		rep.Status, rep.ErrorCode = engine.StatusDegraded, "edge_unreachable"
	} else if r.healthURL != "" {
		if err := checkDataPlane(ctx, r.ln.Addr().String(), r.healthURL); err != nil {
			if ctx.Err() != nil {
				return rep, ctx.Err()
			}
			rep.Status, rep.ErrorCode = engine.StatusDegraded, "data_plane_unreachable"
		} else {
			rep.Status = engine.StatusHealthy
		}
	} else {
		rep.Status = engine.StatusHealthy
	}
	rep.Latency = time.Since(start)
	return rep, nil
}

func (a *Adapter) Stop(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	a.mu.Lock()
	r := a.run
	a.run = nil
	a.mu.Unlock()
	if r == nil {
		return nil
	}
	r.cancel()
	return r.ln.Close()
}
