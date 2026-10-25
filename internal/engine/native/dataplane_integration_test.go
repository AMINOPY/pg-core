package native

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AMINOPY/pg-core/internal/engine"
	"github.com/AMINOPY/pg-core/internal/fronting"
)

func TestAdapterHealthChecksConfiguredDataPlane(t *testing.T) {
	e := newEnv(t)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	a := New()
	a.Fronting = &fronting.Settings{SNI: "example.com", EdgeIPs: []string{"127.0.0.1"}}
	profile, err := json.Marshal(Profile{
		Key: "secret", Port: e.port, Listen: "127.0.0.1:0",
		TimeoutMS: 2000, HealthURL: target.URL + "/health",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := a.Start(ctx, profile); err != nil {
		t.Fatal(err)
	}
	defer a.Stop(ctx)
	a.mu.Lock()
	a.run.dialer.TLSConfig = &tls.Config{RootCAs: e.pool}
	a.mu.Unlock()

	report, err := a.Health(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != engine.StatusHealthy {
		t.Fatalf("data-plane health = %+v, want healthy", report)
	}
}

func TestAdapterRejectsUnsafeHealthURL(t *testing.T) {
	a := New()
	for _, raw := range []string{
		`{"key":"k","server":"edge.example.com","health_url":"http://example.com/health"}`,
		`{"key":"k","server":"edge.example.com","health_url":"https://user:pass@example.com/health"}`,
	} {
		if err := a.ValidateConfig(context.Background(), []byte(raw)); err == nil {
			t.Errorf("accepted unsafe profile %s", raw)
		}
	}
}
