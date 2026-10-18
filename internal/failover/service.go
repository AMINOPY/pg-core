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
		rollbackErr := s.rollback(decision.Target.ID, currentID)
		s.controller.CancelSwitch()
		if rollbackErr != nil {
			return outcome, fmt.Errorf("verify candidate %q: %v; rollback failed: %w", decision.Target.ID, err, rollbackErr)
		}
		return outcome, fmt.Errorf("verify candidate %q: %w (previous candidate restored)", decision.Target.ID, err)
	}
	if err := s.controller.ConfirmSwitch(decision.Target.ID, s.now()); err != nil {
		rollbackErr := s.rollback(decision.Target.ID, currentID)
		if rollbackErr != nil {
			return outcome, fmt.Errorf("confirm candidate switch: %v; rollback failed: %w", err, rollbackErr)
		}
		return outcome, fmt.Errorf("confirm candidate switch: %w", err)
	}
	outcome.Switched = true
	return outcome, nil
}

func (s *Service) rollback(from, to string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.switcher.Rollback(ctx, from, to)
}

// Run periodically samples the current candidate and executes failover decisions.
// The callbacks must return current state owned by the application; the service
// does not infer connection state from process liveness alone.
func (s *Service) Run(
	ctx context.Context,
	interval time.Duration,
	currentID func() string,
	targets func() []health.Target,
	onResult func(Outcome, error),
) error {
	if interval <= 0 || currentID == nil || targets == nil {
		return errors.New("positive interval and state callbacks are required")
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			outcome, err := s.RunOnce(ctx, currentID(), targets())
			if onResult != nil {
				onResult(outcome, err)
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
	}
}
