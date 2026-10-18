package main

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/AMINOPY/pg-core/internal/httpapi"
)

func main() {
	apiKey := os.Getenv("CHIMERA_API_KEY")
	if strings.TrimSpace(apiKey) == "" {
		log.Fatal("CHIMERA_API_KEY must be set; refusing to start without authentication")
	}
	addr := strings.TrimSpace(os.Getenv("CHIMERA_API_ADDR"))
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		log.Fatalf("invalid CHIMERA_API_ADDR: %v", err)
	}
	origins := splitCSV(os.Getenv("CHIMERA_ALLOWED_ORIGINS"))
	handler, err := httpapi.New(apiKey, origins)
	if err != nil {
		log.Fatalf("configure API: %v", err)
	}
	server := httpapi.RecommendedServer(addr, handler)

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-stopCtx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown: %v", err)
		}
	}()

	log.Printf("Chimera API listening on %s (manifest validation only)", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, context.Canceled) && err.Error() != "http: Server closed" {
		log.Fatalf("serve API: %v", err)
	}
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
