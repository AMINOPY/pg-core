package netrecovery

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrAlreadyActive = errors.New("network recovery session is already active")
	ErrNotActive     = errors.New("network recovery session is not active")
	ErrSnapshotEmpty = errors.New("network backend returned an empty snapshot")
)

// Snapshot is an opaque, platform-specific copy of the host network state
// captured before the tunnel applies changes. A backend should include every
// resource it owns (routes, DNS, interface settings, and firewall changes).
type Snapshot struct {
	Data []byte
}

// DesiredState is an opaque, validated network configuration for the backend.
type DesiredState struct {
	Data []byte
}

// Backend applies and restores host networking. Restore should be idempotent.
// Verify must confirm the captured state is restored, not merely that commands
// to restore it were issued.
type Backend interface {
	Capture(context.Context) (Snapshot, error)
	Apply(context.Context, DesiredState) error
	Restore(context.Context, Snapshot) error
	Verify(context.Context, Snapshot) error
}

type phase uint8

const (
	phaseIdle phase = iota
	phaseActive
	phaseRestoring
)

// Session makes network changes transactional: it captures the pre-existing
// state before Apply, restores on Apply failure, and keeps the snapshot until
// restoration and verification both succeed so callers can retry cleanup.
type Session struct {
	mu       sync.Mutex
	backend  Backend
	snapshot *Snapshot
	phase    phase
}

func New(backend Backend) (*Session, error) {
	if backend == nil {
		return nil, errors.New("network recovery backend is required")
	}
	return &Session{backend: backend}, nil
}

// Start captures network state before changing it. If Apply partially fails,
// Start immediately attempts rollback. If rollback also fails, the snapshot is
// retained and Stop can retry restoration.
func (s *Session) Start(ctx context.Context, desired DesiredState) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if s == nil {
		return errors.New("network recovery session is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase != phaseIdle {
		return ErrAlreadyActive
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	snapshot, err := s.backend.Capture(ctx)
	if err != nil {
		return fmt.Errorf("capture host network state: %w", err)
	}
	if len(snapshot.Data) == 0 {
		return ErrSnapshotEmpty
	}
	owned := Snapshot{Data: append([]byte(nil), snapshot.Data...)}
	s.snapshot = &owned
	if err := s.backend.Apply(ctx, desired); err != nil {
		applyErr := fmt.Errorf("apply tunnel network state: %w", err)
		if rollbackErr := s.restoreLocked(ctx); rollbackErr != nil {
			return errors.Join(applyErr, fmt.Errorf("rollback after apply failure: %w", rollbackErr))
		}
		return applyErr
	}
	s.phase = phaseActive
	return nil
}

// Stop restores and verifies the original host network state. On any restore or
// verification error it retains the snapshot and stays recoverable; call Stop
// again after the backend/environment becomes available.
func (s *Session) Stop(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if s == nil {
		return errors.New("network recovery session is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase == phaseIdle || s.snapshot == nil {
		return ErrNotActive
	}
	return s.restoreLocked(ctx)
}

// Recover is the explicit failure path for engine exit, health failure, or
// disconnect. The network restoration result is returned together with cause.
func (s *Session) Recover(ctx context.Context, cause error) error {
	if cause == nil {
		cause = errors.New("network engine failed")
	}
	restoreErr := s.Stop(ctx)
	if restoreErr != nil {
		return errors.Join(cause, fmt.Errorf("restore host network state: %w", restoreErr))
	}
	return cause
}

// Active reports whether the session still owns a captured snapshot, including
// when a previous restoration attempt failed and must be retried.
func (s *Session) Active() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshot != nil
}

func (s *Session) restoreLocked(ctx context.Context) error {
	if s.snapshot == nil {
		return ErrNotActive
	}
	s.phase = phaseRestoring
	snapshot := Snapshot{Data: append([]byte(nil), s.snapshot.Data...)}
	if err := s.backend.Restore(ctx, snapshot); err != nil {
		s.phase = phaseActive
		return fmt.Errorf("restore host network state: %w", err)
	}
	if err := s.backend.Verify(ctx, snapshot); err != nil {
		s.phase = phaseActive
		return fmt.Errorf("verify restored host network state: %w", err)
	}
	s.snapshot = nil
	s.phase = phaseIdle
	return nil
}
