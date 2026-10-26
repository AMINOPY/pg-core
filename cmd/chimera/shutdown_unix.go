//go:build !windows

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// shutdownContext cancels on an interactive interrupt or a normal POSIX
// termination request so engine adapters get a chance to stop child processes
// and clean up private configuration before the CLI exits.
func shutdownContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}
