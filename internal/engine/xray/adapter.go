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
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
	"github.com/AMINOPY/pg-core/internal/fronting"
)

const maxConfigBytes = 4 << 20

var ErrAlreadyRunning = errors.New("xray process is already running")

type processState struct {
	cmd  *exec.Cmd
	dir  string
	done chan struct{}
}

type Adapter struct {
	Binary   string
	Fronting *fronting.Settings

	mu   sync.Mutex
	proc *processState
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
	prepared, err := a.applyFronting(cfg)
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
	if err := dec.Decode(&root); err != nil {
		return nil, errors.New("xray profile is not a valid JSON object")
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

func (a *Adapter) Start(ctx context.Context, cfg []byte) error {
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
	state := &processState{cmd: cmd, dir: dir, done: make(chan struct{})}

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
		report.Status = engine.StatusHealthy
	}
	report.Latency = time.Since(start)
	return report, nil
}

func (a *Adapter) Stop(ctx context.Context) error {
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

func proxyAddress(cfg []byte) (string, error) {
	var root struct {
		Inbounds []struct {
			Listen   string `json:"listen"`
			Port     int    `json:"port"`
			Protocol string `json:"protocol"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(cfg, &root); err != nil {
		return "", errors.New("invalid xray JSON config")
	}
	for _, in := range root.Inbounds {
		if !strings.EqualFold(in.Protocol, "socks") || in.Port < 1 || in.Port > 65535 {
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
	if err := ctx.Err(); err != nil {
		return err
	}
	conn, err := (&net.Dialer{Timeout: 500 * time.Millisecond}).DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(1 * time.Second))
	if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
		return err
	}
	reply := make([]byte, 2)
	if _, err := readFull(conn, reply); err != nil {
		return err
	}
	if reply[0] != 5 || reply[1] != 0 {
		return errors.New("SOCKS5 proxy does not allow no-auth negotiation")
	}
	host, portText, err := net.SplitHostPort(target)
	if err != nil {
		return errors.New("invalid SOCKS target address")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("invalid SOCKS target port")
	}
	ip := net.ParseIP(host)
	var request []byte
	if ip4 := ip.To4(); ip4 != nil {
		request = append([]byte{5, 1, 0, 1}, ip4...)
	} else if ip6 := ip.To16(); ip6 != nil {
		request = append([]byte{5, 1, 0, 4}, ip6...)
	} else {
		if len(host) == 0 || len(host) > 255 {
			return errors.New("invalid SOCKS target hostname")
		}
		request = append([]byte{5, 1, 0, 3, byte(len(host))}, []byte(host)...)
	}
	request = append(request, byte(port>>8), byte(port))
	if _, err := conn.Write(request); err != nil {
		return err
	}
	head := make([]byte, 4)
	if _, err := readFull(conn, head); err != nil {
		return err
	}
	if head[0] != 5 || head[1] != 0 {
		return errors.New("SOCKS5 connect request failed")
	}
	n := 0
	switch head[3] {
	case 1:
		n = 4
	case 4:
		n = 16
	case 3:
		length := make([]byte, 1)
		if _, err := readFull(conn, length); err != nil {
			return err
		}
		n = int(length[0])
	default:
		return errors.New("invalid SOCKS5 address type")
	}
	tail := make([]byte, n+2)
	_, err = readFull(conn, tail)
	return err
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		got, err := conn.Read(buf[n:])
		n += got
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
