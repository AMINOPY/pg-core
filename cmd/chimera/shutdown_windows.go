//go:build windows

package main

import (
	"context"
	"os"
	"os/signal"
)

// shutdownContext handles the console interrupt available to this CLI on
// Windows. OS-specific service shutdown integration remains the host's job.
func shutdownContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}
