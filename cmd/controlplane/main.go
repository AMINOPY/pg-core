// Command controlplane runs the PG CORE remote-control API on loopback.
// Place it behind a TLS-terminating reverse proxy for remote clients.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/AMINOPY/pg-core/internal/controlplane"
)

func main() {
	if err := run(); err != nil {
		log.Printf("control-plane stopped: %v", err)
		os.Exit(1)
	}
}

func run() error {
	rawCredentials := strings.TrimSpace(os.Getenv("PGCORE_CONTROL_PLANE_CREDENTIALS"))
	if rawCredentials == "" {
		return errors.New("PGCORE_CONTROL_PLANE_CREDENTIALS must contain a JSON array of credentials")
	}
	var credentials []controlplane.Credential
	dec := json.NewDecoder(strings.NewReader(rawCredentials))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&credentials); err != nil {
		return errors.New("PGCORE_CONTROL_PLANE_CREDENTIALS is invalid JSON")
	}
	if len(credentials) == 0 {
		return errors.New("at least one API credential is required")
	}

	addr := strings.TrimSpace(os.Getenv("PGCORE_CONTROL_PLANE_ADDR"))
	if addr == "" {
		addr = "127.0.0.1:8788"
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return errors.New("PGCORE_CONTROL_PLANE_ADDR must be a host:port address")
	}
	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return errors.New("control-plane binds to loopback only; expose it through a TLS reverse proxy")
		}
	}

	statePath := strings.TrimSpace(os.Getenv("PGCORE_CONTROL_PLANE_DB"))
	if statePath == "" {
		statePath = "./data/pgcore-controlplane.json"
	}
	store, err := controlplane.NewFileStore(statePath)
	if err != nil {
		return err
	}
	api, err := controlplane.NewServer(store, credentials)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           api,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()
	log.Printf("PG CORE control-plane listening on %s (loopback only)", addr)
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
