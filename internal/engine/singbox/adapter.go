// Package singbox adapts the upstream sing-box executable to the engine contract.
// It manages only the child process and its private config file; it does not
// change OS routes, install DNS settings, or configure a system TUN interface.
package singbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
	"github.com/AMINOPY/pg-core/internal/fronting"
)

const maxConfigBytes = 4 << 20

var ErrAlreadyRunning = errors.New("sing-box process is already running")

type processState struct {
	cmd     *exec.Cmd
	dir     string
	done    chan struct{}
	waitErr error
}

// Adapter runs an installed sing-box executable. Binary may be an absolute path
// or a name resolvable on PATH. Output is not forwarded because upstream
// diagnostics may contain sensitive configuration details.
type Adapter struct {
	Binary string
	// Fronting is the optional CDN/SNI feature (SNI + edge IP list). When it is
	// enabled it is applied to the profile before validation and start.
	Fronting *fronting.Settings

	mu   sync.Mutex
	proc *processState
}

func New(binary string) *Adapter { return &Adapter{Binary: binary} }

func (a *Adapter) ID() string { return "sing-box" }

func (a *Adapter) Capabilities() engine.Capabilities {
	return engine.Capabilities{TUN: true, DNS: true, Routing: true, HealthChecks: true}
}

func (a *Adapter) ValidateConfig(ctx context.Context, cfg []byte) error {
	if err := validateInput(ctx, a.Binary, cfg); err != nil {
		return err
	}
	cfg, err := a.applyFronting(cfg)
	if err != nil {
		return err
	}
	return a.checkProfile(ctx, cfg)
}

func (a *Adapter) applyFronting(cfg []byte) ([]byte, error) {
	if a.Fronting == nil || !a.Fronting.Enabled() {
		return cfg, nil
	}
	out, err := fronting.Apply(cfg, *a.Fronting)
	if err != nil {
		return nil, fmt.Errorf("apply CDN/SNI: %w", err)
	}
	return out, nil
}

// checkProfile runs `sing-box check` on an already prepared profile.
func (a *Adapter) checkProfile(ctx context.Context, cfg []byte) error {
	dir, configPath, err := writePrivateConfig(cfg)
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	cmd := exec.CommandContext(ctx, a.Binary, "check", "-c", configPath)
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("sing-box config check failed")
	}
	return nil
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

	cfg, err := a.applyFronting(cfg)
	if err != nil {
		return err
	}
	if err := a.checkProfile(ctx, cfg); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	dir, configPath, err := writePrivateConfig(cfg)
	if err != nil {
		return err
	}

	cmd := exec.Command(a.Binary, "run", "-c", configPath)
	// Do not attach stdout/stderr: they could leak configuration secrets.
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("start sing-box process: %w", err)
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
		err := cmd.Wait()
		a.mu.Lock()
		state.waitErr = err
		close(state.done)
		a.mu.Unlock()
	}()
	return nil
}

func (a *Adapter) Health(ctx context.Context) (engine.HealthReport, error) {
	start := time.Now()
	report := engine.HealthReport{EngineID: "sing-box", CheckedAt: time.Now().UTC()}
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
		return fmt.Errorf("remove private sing-box config: %w", err)
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
	if binary == "" {
		return errors.New("sing-box binary path is required")
	}
	if len(cfg) == 0 {
		return errors.New("sing-box config must not be empty")
	}
	if len(cfg) > maxConfigBytes {
		return fmt.Errorf("sing-box config exceeds %d bytes", maxConfigBytes)
	}
	return nil
}

func writePrivateConfig(cfg []byte) (dir, configPath string, err error) {
	dir, err = os.MkdirTemp("", "chimera-singbox-*")
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
		return cleanup(fmt.Errorf("write private sing-box config: %w", err))
	}
	return dir, configPath, nil
}
