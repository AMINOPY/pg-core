package diagnostics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBenchmarkerBoundsTransferAndMeasures(t *testing.T) {
	payload := strings.Repeat("x", 4096)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()

	result := (Benchmarker{
		Timeout:  time.Second,
		MaxBytes: 1024,
		Client:   server.Client(),
	}).Benchmark(context.Background(), server.URL+"/sample")

	if result.Error != "" {
		t.Fatalf("Benchmark error: %s", result.Error)
	}
	if result.HTTPStatus != http.StatusOK {
		t.Fatalf("status=%d, want %d", result.HTTPStatus, http.StatusOK)
	}
	if result.BytesRead != 1024 {
		t.Fatalf("BytesRead=%d, want bounded 1024", result.BytesRead)
	}
	if result.TotalLatency <= 0 || result.ThroughputMbps <= 0 {
		t.Fatalf("expected positive timing and throughput: %+v", result)
	}
}

func TestBenchmarkerRejectsUnsafeURLShapes(t *testing.T) {
	tests := []string{
		"http://example.com",
		"ftp://example.com/file",
		"https://user:password@example.com/",
		"https://example.com/#fragment",
		"/relative/path",
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			got := (Benchmarker{}).Benchmark(context.Background(), raw)
			if got.Error == "" {
				t.Fatalf("Benchmark(%q) unexpectedly succeeded", raw)
			}
		})
	}
}

func TestBenchmarkerDoesNotFollowRedirects(t *testing.T) {
	targetCalled := false
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	redirect := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirect.Close()

	result := (Benchmarker{Client: redirect.Client()}).Benchmark(context.Background(), redirect.URL)
	if result.HTTPStatus != http.StatusFound {
		t.Fatalf("status=%d, want %d", result.HTTPStatus, http.StatusFound)
	}
	if result.Error == "" {
		t.Fatal("expected non-2xx status to be reported")
	}
	if targetCalled {
		t.Fatal("benchmark followed a redirect")
	}
}

func TestBenchmarkerHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := (Benchmarker{}).Benchmark(ctx, "https://example.com")
	if result.Error == "" {
		t.Fatal("expected cancelled context to fail")
	}
}

func TestBenchmarkerRedactsPathAndQueryFromResult(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	result := (Benchmarker{Client: server.Client()}).Benchmark(
		context.Background(), server.URL+"/private-path?token=do-not-return",
	)
	if strings.Contains(result.URL, "private-path") || strings.Contains(result.URL, "do-not-return") {
		t.Fatalf("diagnostic URL leaked path or query: %q", result.URL)
	}
	if result.Error != "" {
		t.Fatalf("Benchmark error: %s", result.Error)
	}
}

func TestBenchmarkerCapsConfiguredBytesAndTimeout(t *testing.T) {
	b := Benchmarker{MaxBytes: maxBenchmarkBytes + 1, Timeout: time.Hour}
	if got := b.maxBytes(); got != maxBenchmarkBytes {
		t.Fatalf("maxBytes=%d, want %d", got, maxBenchmarkBytes)
	}
	if got := b.timeout(); got != defaultBenchmarkTimeout {
		t.Fatalf("timeout=%s, want %s", got, defaultBenchmarkTimeout)
	}
}
