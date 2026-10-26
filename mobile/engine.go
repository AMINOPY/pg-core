// Package mobile exposes the native SOCKS engine to an Android/JNI binding layer.
// It deliberately does not claim to implement TUN packet forwarding: a platform
// tun2socks backend must feed packets to the local SOCKS listener separately.
package mobile

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"syscall"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine/native"
)

// SocketProtector must synchronously call Android VpnService.protect(fd).
// Returning false rejects the socket and fails the outbound connection closed.
type SocketProtector interface {
	Protect(fd int) bool
}

type socketControl struct {
	protector SocketProtector
}

func (c socketControl) control(_ string, _ string, raw syscall.RawConn) error {
	if c.protector == nil {
		return errors.New("Android socket protector is required")
	}
	if raw == nil {
		return errors.New("outbound socket handle is unavailable")
	}
	var protectErr error
	if err := raw.Control(func(fd uintptr) {
		if !c.protector.Protect(int(fd)) {
			protectErr = errors.New("VpnService.protect rejected outbound socket")
		}
	}); err != nil {
		return fmt.Errorf("access outbound socket: %w", err)
	}
	return protectErr
}

// Engine owns the native TLS/WebSocket-to-SOCKS engine lifecycle. The caller
// must still connect an Android TUN/tun2socks data plane to Addr().
type Engine struct {
	mu      sync.Mutex
	adapter *native.Adapter
}

// NewEngine creates an idle engine. No listener or network connection is opened.
func NewEngine() *Engine { return &Engine{} }

// Start starts a loopback-only SOCKS listener. profileJSON follows
// internal/engine/native.Profile JSON fields. A non-nil protector is mandatory.
func (e *Engine) Start(profileJSON string, protector SocketProtector) error {
	if e == nil {
		return errors.New("engine is required")
	}
	if protector == nil {
		return errors.New("Android socket protector is required")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.adapter != nil {
		return errors.New("mobile engine is already started")
	}
	a := native.New()
	control := socketControl{protector: protector}
	a.Control = control.control
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.ValidateConfig(ctx, []byte(profileJSON)); err != nil {
		return fmt.Errorf("validate native profile: %w", err)
	}
	if err := a.Start(ctx, []byte(profileJSON)); err != nil {
		return fmt.Errorf("start native engine: %w", err)
	}
	e.adapter = a
	return nil
}

// Addr returns the loopback SOCKS endpoint, or an empty string before Start.
func (e *Engine) Addr() string {
	if e == nil {
		return ""
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.adapter == nil {
		return ""
	}
	return e.adapter.Addr()
}

// Health requires a real configured HealthURL; a TLS/WebSocket handshake alone
// is not considered proof that user traffic traverses the data plane.
func (e *Engine) Health(timeoutMillis int64) error {
	if e == nil {
		return errors.New("engine is required")
	}
	if timeoutMillis <= 0 || timeoutMillis > 60000 {
		return errors.New("health timeout must be between 1 and 60000 ms")
	}
	e.mu.Lock()
	a := e.adapter
	e.mu.Unlock()
	if a == nil {
		return errors.New("mobile engine is not started")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMillis)*time.Millisecond)
	defer cancel()
	report, err := a.Health(ctx)
	if err != nil {
		return err
	}
	if report.Status != "healthy" {
		return fmt.Errorf("native engine health is not verified: %s", report.ErrorCode)
	}
	return nil
}

// Stop is idempotent and keeps ownership if the underlying listener/sessions
// have not shut down before the supplied timeout.
func (e *Engine) Stop(timeoutMillis int64) error {
	if e == nil {
		return errors.New("engine is required")
	}
	if timeoutMillis <= 0 || timeoutMillis > 60000 {
		return errors.New("stop timeout must be between 1 and 60000 ms")
	}
	e.mu.Lock()
	a := e.adapter
	e.mu.Unlock()
	if a == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMillis)*time.Millisecond)
	defer cancel()
	if err := a.Stop(ctx); err != nil {
		return err
	}
	e.mu.Lock()
	if e.adapter == a {
		e.adapter = nil
	}
	e.mu.Unlock()
	return nil
}
