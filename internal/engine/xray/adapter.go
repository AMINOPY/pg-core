// Package xray adapts an installed Xray executable to the shared engine contract.
// It owns only the child process and its private configuration; OS routes and
// system DNS remain the host application's responsibility.
package xray

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
	"github.com/AMINOPY/pg-core/internal/engine/socksprobe"
	"github.com/AMINOPY/pg-core/internal/fronting"
)

const maxConfigBytes = 4 << 20

var ErrAlreadyRunning = errors.New("xray process is already running")

type processState struct {
	cmd          *exec.Cmd
	dir          string
	done         chan struct{}
	proxyAddr    string
	healthTarget string
}

type Adapter struct {
	Binary   string
	Fronting *fronting.Settings

	// LocalSOCKSPort opts into a loopback-only, no-auth SOCKS5 inbound for the
	// embedding app. Zero leaves the supplied profile unchanged.
	LocalSOCKSPort int

	// HealthTarget optionally enables a TCP data-path probe through a loopback,
	// no-auth SOCKS5 inbound (for example, "example.com:443"). Empty preserves
	// process-liveness-only health. Use a stable endpoint you are authorized to test.
	HealthTarget string

	// lifecycle serializes Start/Stop so concurrent callers cannot spawn
	// duplicate children or remove a config directory still in use.
	lifecycle sync.Mutex
	mu        sync.Mutex
	proc      *processState
}

func New(binary string) *Adapter { return &Adapter{Binary: binary} }

func (a *Adapter) ID() string { return "xray" }

func (a *Adapter) Capabilities() engine.Capabilities {
	// Xray may be configured for TUN through platform-specific wrappers, but
	// this adapter does not create or manage a TUN device itself.
	return engine.Capabilities{HealthChecks: true}
}

func (a *Adapter) ValidateConfig(ctx context.Context, cfg []byte) error {
	if err := validateInput(ctx, a.Binary, cfg); err != nil {
		return err
	}
	prepared, err := a.prepareConfig(cfg)
	if err != nil {
		return err
	}
	return a.validatePreparedConfig(ctx, prepared)
}

func (a *Adapter) validatePreparedConfig(ctx context.Context, prepared []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir, configPath, err := writePrivateConfig(prepared)
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	cmd := exec.CommandContext(ctx, a.Binary, "run", "-test", "-config", configPath)
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Do not return executable output: it may contain credentials or config.
		return errors.New("xray config validation failed")
	}
	return nil
}

func (a *Adapter) applyFronting(cfg []byte) ([]byte, error) {
	if a.Fronting == nil || !a.Fronting.Enabled() {
		return cfg, nil
	}
	if err := a.Fronting.Validate(); err != nil {
		return nil, err
	}
	ips, _ := fronting.ParseIPs(a.Fronting.EdgeIPs)
	if len(ips) != 1 {
		return nil, errors.New("xray fronting requires exactly one edge IP per profile attempt")
	}
	sni := strings.TrimSpace(a.Fronting.SNI)

	dec := json.NewDecoder(bytes.NewReader(cfg))
	dec.UseNumber()
	var root map[string]any
	if err := dec.Decode(&root); err != nil || root == nil {
		return nil, errors.New("xray profile is not a valid JSON object")
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return nil, errors.New("xray profile contains trailing JSON data")
	}
	outbounds, ok := root["outbounds"].([]any)
	if !ok {
		return nil, errors.New("xray profile must contain outbounds")
	}
	changed := 0
	for _, item := range outbounds {
		ob, ok := item.(map[string]any)
		if !ok {
			continue
		}
		stream, ok := ob["streamSettings"].(map[string]any)
		if !ok {
			continue
		}
		security, _ := stream["security"].(string)
		if !strings.EqualFold(security, "tls") {
			continue
		}
		settings, ok := ob["settings"].(map[string]any)
		if !ok {
			continue
		}
		vnext, ok := settings["vnext"].([]any)
		if !ok || len(vnext) != 1 {
			continue
		}
		server, ok := vnext[0].(map[string]any)
		if !ok {
			continue
		}
		tls, ok := stream["tlsSettings"].(map[string]any)
		if !ok {
			tls = map[string]any{}
			stream["tlsSettings"] = tls
		}
		server["address"] = ips[0]
		tls["serverName"] = sni
		changed++
	}
	if changed == 0 {
		return nil, errors.New("fronting is configured but no single-server TLS outbound was found")
	}
	return json.Marshal(root)
}


 // prepareConfig composes core-owned transport settings in a deterministic order.
func (a *Adapter) prepareConfig(cfg []byte) ([]byte, error) {
	prepared, err := a.applyFronting(cfg)
	if err != nil {
		return nil, err
	}
	return applyLocalSOCKS(prepared, a.LocalSOCKSPort)
}

// applyLocalSOCKS adds an app-facing SOCKS inbound bound strictly to IPv4
// loopback. It never exposes an unauthenticated proxy on a LAN/public interface.
func applyLocalSOCKS(cfg []byte, port int) ([]byte, error) {
	if port == 0 {
		return cfg, nil
	}
	if port < 1 || port > 65535 {
		return nil, errors.New("local SOCKS port must be in 1..65535")
	}
	dec := json.NewDecoder(bytes.NewReader(cfg))
	dec.UseNumber()
	var root map[string]any
	if err := dec.Decode(&root); err != nil || root == nil {
		return nil, errors.New("xray profile is not a valid JSON object")
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return nil, errors.New("xray profile contains trailing JSON data")
	}
	inbounds, ok := root["inbounds"].([]any)
	if !ok {
		inbounds = []any{}
	}
	for _, item := range inbounds {
		in, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rawPort, ok := in["port"].(json.Number)
		if !ok {
			continue
		}
		existingPort, err := strconv.Atoi(string(rawPort))
		if err != nil || existingPort != port {
			continue
		}
		listen, _ := in["listen"].(string)
		protocol, _ := in["protocol"].(string)
		settings, _ := in["settings"].(map[string]any)
		auth, _ := settings["auth"].(string)
		if listen == "127.0.0.1" && protocol == "socks" && auth == "noauth" {
			return json.Marshal(root)
		}
		return nil, errors.New("local SOCKS port conflicts with an existing Xray inbound")
	}
	root["inbounds"] = append(inbounds, map[string]any{
		"tag": "pgcore-local-socks",
		"listen": "127.0.0.1",
		"port": port,
		"protocol": "socks",
		"settings": map[string]any{"auth": "noauth", "udp": true},
	})
	return json.Marshal(root)
}

func (a *Adapter) Start(ctx context.Context, cfg []byte) error {
	a.lifecycle.Lock()
	defer a.lifecycle.Unlock()

	if err := validateInput(ctx, a.Binary, cfg); err != nil {
		return err
	}
	a.mu.Lock()
	if a.proc != nil {
		select {
		case <-a.proc.done:
			_ = os.RemoveAll(a.proc.dir)
			a.proc = nil
		default:
			a.mu.Unlock()
			return ErrAlreadyRunning
		}
	}
	a.mu.Unlock()

	prepared, err := a.applyFronting(cfg)
	if err != nil {
		return err
	}
	proxyAddr := ""
	healthTarget := strings.TrimSpace(a.HealthTarget)
	if healthTarget != "" {
		if err := validateHealthTarget(healthTarget); err != nil {
			return err
		}
		proxyAddr, err = proxyAddress(prepared)
		if err != nil {
			return fmt.Errorf("data-path health requires a loopback SOCKS inbound: %w", err)
		}
	}
	if err := a.validatePreparedConfig(ctx, prepared); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	dir, configPath, err := writePrivateConfig(prepared)
	if err != nil {
		return err
	}
	cmd := exec.Command(a.Binary, "run", "-config", configPath)
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(dir)
		return errors.New("start xray process failed")
	}
	state := &processState{cmd: cmd, dir: dir, done: make(chan struct{}), proxyAddr: proxyAddr, healthTarget: healthTarget}

	a.mu.Lock()
	if a.proc != nil {
		select {
		case <-a.proc.done:
			_ = os.RemoveAll(a.proc.dir)
			a.proc = nil
		default:
			a.mu.Unlock()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			_ = os.RemoveAll(dir)
			return ErrAlreadyRunning
		}
	}
	a.proc = state
	a.mu.Unlock()

	go func() {
		_ = cmd.Wait()
		a.mu.Lock()
		close(state.done)
		a.mu.Unlock()
	}()
	return nil
}

func (a *Adapter) Health(ctx context.Context) (engine.HealthReport, error) {
	start := time.Now()
	if ctx == nil {
		return engine.HealthReport{}, errors.New("context is required")
	}
	report := engine.HealthReport{EngineID: a.ID(), CheckedAt: time.Now().UTC()}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	a.mu.Lock()
	state := a.proc
	a.mu.Unlock()
	if state == nil {
		report.Status = engine.StatusStopped
		report.ErrorCode = "process_not_started"
		report.Latency = time.Since(start)
		return report, nil
	}
	select {
	case <-ctx.Done():
		return report, ctx.Err()
	case <-state.done:
		report.Status = engine.StatusFailed
		report.ErrorCode = "process_exited"
	default:
		if state.healthTarget == "" {
			report.Status = engine.StatusDegraded
			report.ErrorCode = "process_liveness_only"
		} else {
			report.Status = engine.StatusHealthy
		}
	}
	if report.Status == engine.StatusHealthy && state.healthTarget != "" {
		if err := probeViaSOCKS(ctx, state.proxyAddr, state.healthTarget); err != nil {
			if ctx.Err() != nil {
				return report, ctx.Err()
			}
			report.Status = engine.StatusFailed
			report.ErrorCode = "data_path_probe_failed"
		}
	}
	report.Latency = time.Since(start)
	return report, nil
}

func (a *Adapter) Stop(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	a.lifecycle.Lock()
	defer a.lifecycle.Unlock()

	a.mu.Lock()
	state := a.proc
	a.mu.Unlock()
	if state == nil {
		return nil
	}
	select {
	case <-state.done:
	default:
		if err := state.cmd.Process.Signal(os.Interrupt); err != nil {
			select {
			case <-state.done:
			default:
				_ = state.cmd.Process.Kill()
			}
		}
		select {
		case <-state.done:
		case <-ctx.Done():
			_ = state.cmd.Process.Kill()
			<-state.done
			a.clearProcess(state)
			_ = os.RemoveAll(state.dir)
			return ctx.Err()
		}
	}
	a.clearProcess(state)
	if err := os.RemoveAll(state.dir); err != nil {
		return fmt.Errorf("remove private xray config: %w", err)
	}
	return nil
}

func (a *Adapter) clearProcess(state *processState) {
	a.mu.Lock()
	if a.proc == state {
		a.proc = nil
	}
	a.mu.Unlock()
}

func validateInput(ctx context.Context, binary string, cfg []byte) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(binary) == "" {
		return errors.New("xray binary path is required")
	}
	if len(cfg) == 0 {
		return errors.New("xray config must not be empty")
	}
	if len(cfg) > maxConfigBytes {
		return fmt.Errorf("xray config exceeds %d bytes", maxConfigBytes)
	}
	return nil
}

func writePrivateConfig(cfg []byte) (dir, configPath string, err error) {
	dir, err = os.MkdirTemp("", "chimera-xray-*")
	if err != nil {
		return "", "", fmt.Errorf("create private config directory: %w", err)
	}
	cleanup := func(e error) (string, string, error) {
		_ = os.RemoveAll(dir)
		return "", "", e
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return cleanup(fmt.Errorf("secure private config directory: %w", err))
	}
	configPath = filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, cfg, 0o600); err != nil {
		return cleanup(fmt.Errorf("write private xray config: %w", err))
	}
	return dir, configPath, nil
}

func validateHealthTarget(target string) error {
	return socksprobe.ValidateTarget(target)
}

func proxyAddress(cfg []byte) (string, error) {
	var root struct {
		Inbounds []struct {
			Listen   string `json:"listen"`
			Port     int    `json:"port"`
			Protocol string `json:"protocol"`
			Settings struct {
				Auth     string            `json:"auth"`
				Accounts []json.RawMessage `json:"accounts"`
			} `json:"settings"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(cfg, &root); err != nil {
		return "", errors.New("invalid xray JSON config")
	}
	for _, in := range root.Inbounds {
		if !strings.EqualFold(in.Protocol, "socks") || in.Port < 1 || in.Port > 65535 || (in.Settings.Auth != "" && !strings.EqualFold(in.Settings.Auth, "noauth")) || len(in.Settings.Accounts) != 0 {
			continue
		}
		host := strings.TrimSpace(in.Listen)
		if host == "" {
			host = "127.0.0.1"
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			continue
		}
		return net.JoinHostPort(ip.String(), strconv.Itoa(in.Port)), nil
	}
	return "", errors.New("xray config requires a loopback-only SOCKS inbound")
}

func probeViaSOCKS(ctx context.Context, proxyAddr, target string) error {
	return socksprobe.ProbeTCP(ctx, proxyAddr, target)
}
