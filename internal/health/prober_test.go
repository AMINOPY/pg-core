package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
)

func TestProbeHealthyAndServerError(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	prober, err := NewProber(time.Second, 2)
	if err != nil {
		t.Fatal(err)
	}
	prober.client = server.Client()
	results, err := prober.Probe(context.Background(), []Target{
		{ID: "ok", SubscriptionID: "sub-a", URL: server.URL + "/ok"},
		{ID: "bad", SubscriptionID: "sub-b", URL: server.URL + "/bad"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Candidate.ID != "bad" || results[1].Candidate.ID != "ok" {
		t.Fatalf("results should be sorted by ID: %+v", results)
	}
	if results[0].Candidate.Status != engine.StatusFailed || results[0].Candidate.Latency < 0 {
		t.Fatalf("server error should fail with a measured latency: %+v", results[0])
	}
	if results[1].Candidate.Status != engine.StatusHealthy || results[1].Candidate.Latency < 0 {
		t.Fatalf("healthy response expected: %+v", results[1])
	}
}

func TestProbeRejectsHTTPAndDuplicateIDs(t *testing.T) {
	prober, err := NewProber(time.Second, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prober.Probe(context.Background(), []Target{{ID: "a", URL: "http://example.com"}}); err == nil {
		t.Fatal("expected plain HTTP to be rejected")
	}
	if _, err := prober.Probe(context.Background(), []Target{
		{ID: "a", URL: "https://example.com"},
		{ID: "a", URL: "https://example.org"},
	}); err == nil {
		t.Fatal("expected duplicate IDs to be rejected")
	}
}

func TestProbeHonorsCancelledContext(t *testing.T) {
	prober, err := NewProber(time.Second, 1)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = prober.Probe(ctx, []Target{{ID: "a", URL: "https://example.com"}})
	if err == nil {
		t.Fatal("expected cancellation")
	}
}
