// Package diagnostics provides bounded, opt-in self-healing primitives.
// It deliberately performs no repairs unless an application registers a Repairer.
package diagnostics

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrNoChecker   = errors.New("diagnostics: checker is required")
	ErrNoRepairer  = errors.New("diagnostics: repairer is required")
	ErrRepairLimit = errors.New("diagnostics: repair attempt limit reached")
)

type Finding struct {
	Code        string
	Component   string
	Summary     string
	Recoverable bool
}

type Checker interface {
	Check(context.Context) (Finding, error)
}

type Repairer interface {
	Repair(context.Context, Finding) error
}

type Verifier interface {
	Verify(context.Context, Finding) error
}

type Result struct {
	Finding    Finding
	Attempted  bool
	Repaired   bool
	StartedAt  time.Time
	FinishedAt time.Time
	Err        error
}

type Runner struct {
	Checker     Checker
	Repairer    Repairer
	Verifier    Verifier
	Timeout     time.Duration
	MaxAttempts int
}

func (r Runner) RunOnce(ctx context.Context) Result {
	result := Result{StartedAt: time.Now()}
	defer func() { result.FinishedAt = time.Now() }()
	if r.Checker == nil {
		result.Err = ErrNoChecker
		return result
	}
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}
	finding, err := r.Checker.Check(ctx)
	result.Finding = finding
	if err != nil {
		result.Err = fmt.Errorf("diagnostics check: %w", err)
		return result
	}
	if finding.Code == "" || !finding.Recoverable {
		return result
	}
	if r.Repairer == nil {
		result.Err = ErrNoRepairer
		return result
	}
	limit := r.MaxAttempts
	if limit == 0 {
		limit = 1
	}
	if limit < 1 || limit > 3 {
		result.Err = fmt.Errorf("diagnostics: MaxAttempts must be between 1 and 3")
		return result
	}
	for attempt := 0; attempt < limit; attempt++ {
		result.Attempted = true
		if err := r.Repairer.Repair(ctx, finding); err != nil {
			result.Err = fmt.Errorf("diagnostics repair attempt %d: %w", attempt+1, err)
			continue
		}
		if r.Verifier == nil {
			result.Err = errors.New("diagnostics: verifier required before repair can be marked successful")
			return result
		}
		if err := r.Verifier.Verify(ctx, finding); err == nil {
			result.Repaired = true
			result.Err = nil
			return result
		} else {
			result.Err = fmt.Errorf("diagnostics verification after attempt %d: %w", attempt+1, err)
		}
		if ctx.Err() != nil {
			result.Err = ctx.Err()
			return result
		}
	}
	if result.Err == nil {
		result.Err = ErrRepairLimit
	}
	return result
}
