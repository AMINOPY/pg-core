package orchestrator

import (
	"context"
	"errors"
	"fmt"
)

// SwitchWithRollback stops the current engine before starting its replacement.
// If replacement startup fails and cleanup is confirmed, it attempts to restore
// the current configuration. It returns the current engine ID with a non-nil
// error when rollback succeeds. It never starts two engines with overlapping
// exclusive claims intentionally; Fleet.Start remains the final ownership gate.
//
// The host must still verify end-to-end data-plane connectivity after a switch.
// This coordinates lifecycle only and is not seamless packet-level handover.
func (f *Fleet) SwitchWithRollback(ctx context.Context, current, replacement Candidate) (string, error) {
	if f == nil {
		return "", errors.New("fleet is required")
	}
	if ctx == nil {
		return "", ErrContextRequired
	}
	if current.EngineID == "" || replacement.EngineID == "" {
		return "", errors.New("current and replacement engine IDs are required")
	}
	if current.EngineID == replacement.EngineID {
		return "", errors.New("current and replacement engines must differ")
	}

	status, _, err := f.Status(current.EngineID)
	if err != nil {
		return "", err
	}
	if status == "stopped" {
		return "", fmt.Errorf("current engine %q is stopped", current.EngineID)
	}
	if err := f.Stop(ctx, current.EngineID); err != nil {
		return "", fmt.Errorf("stop current engine %q before switch: %w", current.EngineID, err)
	}

	replacementErr := f.Start(ctx, replacement.EngineID, replacement.Config, replacement.Claims...)
	if replacementErr == nil {
		return replacement.EngineID, nil
	}
	if errors.Is(replacementErr, ErrResourceConflict) {
		return "", fmt.Errorf("replacement %q could not claim resources; current engine remains stopped: %w", replacement.EngineID, replacementErr)
	}

	if errors.Is(replacementErr, ErrResourceStateUncertain) {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
		cleanupErr := f.Stop(cleanupCtx, replacement.EngineID)
		cancel()
		if cleanupErr != nil {
			return "", fmt.Errorf("replacement %q failed and resource cleanup is uncertain; original engine was not restarted: %w", replacement.EngineID, errors.Join(replacementErr, cleanupErr))
		}
	}

	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("replacement %q failed and switch context is cancelled; current engine remains stopped: %w", replacement.EngineID, errors.Join(replacementErr, err))
	}
	if restoreErr := f.Start(ctx, current.EngineID, current.Config, current.Claims...); restoreErr != nil {
		return "", fmt.Errorf("replacement %q failed and original engine %q could not be restored: %w", replacement.EngineID, current.EngineID, errors.Join(replacementErr, restoreErr))
	}
	return current.EngineID, fmt.Errorf("replacement %q failed; original engine %q was restored: %w", replacement.EngineID, current.EngineID, replacementErr)
}
