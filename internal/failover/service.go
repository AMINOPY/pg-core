// Package failover wires bounded health samples to the policy and an
// application-supplied connection switcher. It does not own OS routes or TUN.
package failover

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/AMINOPY/pg-core/internal/health"
	"github.com/AMINOPY/pg-core/internal/policy"
)

var ErrMissingDependency = errors.New("failover service dependency is missing")

type Sampler interface {
	Probe(context.Context, []health.Target) ([]health.ProbeResult, error)
}

// Switcher must implement platform-specific, serialized connection replacement.
// Rollback must restore the previous target if Verify fails.
type Switcher interface {
	Switch(context.Context, string, string) error
	Verify(context.Context, string) error
	Rollback(context.Context, string, string) error
}

type Outcome struct {
	Decision policy.FailoverDecision
	Probes   []health.ProbeResult
	Switched bool
}

type Service struct {
	mu         sync.Mutex
	controller *policy.FailoverController
	sampler    Sampler
	switcher   Switcher
	now        func() time.Time
}

func New(controller *policy.FailoverController, sampler Sampler, switcher Switcher) (*Service, error) {
	if controller == nil || sampler == nil || switcher == nil {
		return nil, ErrMissingDependency
	}
	return &Service{controller: controller, sampler: sampler, switcher: switcher, now: time.Now}, nil
}

// RunOnce samples configured endpoints, asks the policy for a decision, and
// executes it. Calls are serialized to avoid overlapping connection changes.
func (s *Service) RunOnce(ctx context.Context, currentID string, targets []health.Target) (Outcome, error) {
	if s == nil {
		return Outcome{}, ErrMissingDependency
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	probes, err := s.sampler.Probe(ctx, targets)
	if err != nil {
		return Outcome{}, fmt.Errorf("sample candidates: %w", err)
	}
	candidates := make([]policy.ServerCandidate, len(probes))
	for i := range probes {
		candidates[i] = probes[i].Candidate
	}
	decision, err := s.controller.Evaluate(currentID, candidates, s.now())
	if err != nil {
		return Outcome{Probes: probes}, err
	}
	outcome := Outcome{Decision: decision, Probes: probes}
	if !decision.Switch {
		return outcome, nil
	}
	if err := ctx.Err(); err != nil {
		s.controller.CancelSwitch()
		return outcome, err
	}
	if err := s.switcher.Switch(ctx, currentID, decision.Target.ID); err != nil {
		s.controller.CancelSwitch()
		return outcome, fmt.Errorf("switch to candidate %q: %w", decision.Target.ID, err)
	}
	if err := s.switcher.Verify(ctx, decision.Target.ID); err != nil {
		rollbackErr := s.switcher.Rollback(ctx, decision.Target.ID, currentID)
		s.controller.CancelSwitch()
		if rollbackErr != nil {
			return outcome, fmt.Errorf("verify candidate %q: %v; rollback failed: %w", decision.Target.ID, err, rollbackErr)
		}
		return outcome, fmt.Errorf("verify candidate %q: %w (previous candidate restored)", decision.Target.ID, err)
	}
	if err := s.controller.ConfirmSwitch(decision.Target.ID, s.now()); err != nil {
		rollbackErr := s.switcher.Rollback(ctx, decision.Target.ID, currentID)
		if rollbackErr != nil {
			return outcome, fmt.Errorf("confirm candidate switch: %v; rollback failed: %w", err, rollbackErr)
		}
		return outcome, fmt.Errorf("confirm candidate switch: %w", err)
	}
	outcome.Switched = true
	return outcome, nil
}
