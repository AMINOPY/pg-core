package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testManifest = `{
  "schema_version":"1.0",
  "connector_id":"test-connector",
  "connector_version":"1.0.0",
  "capabilities":[{
    "id":"text.summarize",
    "version":"1.0",
    "input_schema":{"type":"object"},
    "output_schema":{"type":"object"},
    "permissions":["model.inference"],
    "risk":"low",
    "execution":"sandboxed"
  }]
}`

func newTestHandler(t *testing.T, origins ...string) http.Handler {
	t.Helper()
	h, err := New("test-secret-key", origins)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestNewRequiresAPIKey(t *testing.T) {
	if _, err := New(" ", nil); err == nil {
		t.Fatal("expected empty API key to be rejected")
	}
}

func TestHealthIsPublic(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestManifestEndpointRequiresBearerToken(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/manifests/validate", strings.NewReader(testManifest))
	req.Header.Set("Content-Type", "application/json")
	newTestHandler(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestManifestEndpointValidatesAndSummarizes(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/manifests/validate", bytes.NewBufferString(testManifest))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-secret-key")
	newTestHandler(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"connector_id":"test-connector"`) {
		t.Fatalf("response missing connector id: %s", rec.Body.String())
	}
}

func TestManifestEndpointRejectsInvalidManifest(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/manifests/validate", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-secret-key")
	newTestHandler(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if strings.Contains(rec.Body.String(), "schema_version") {
		t.Fatalf("response leaks parser details: %s", rec.Body.String())
	}
}

func TestManifestEndpointRejectsWrongContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/manifests/validate", strings.NewReader(testManifest))
	req.Header.Set("Authorization", "Bearer test-secret-key")
	req.Header.Set("Content-Type", "text/plain")
	newTestHandler(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestCORSRequiresExactOrigin(t *testing.T) {
	h := newTestHandler(t, "https://app.example.com")
	allowed := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://app.example.com")
	h.ServeHTTP(allowed, req)
	if allowed.Code != http.StatusOK || allowed.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Fatalf("allowed origin failed: status=%d headers=%v", allowed.Code, allowed.Header())
	}
	blocked := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://evil.example")
	h.ServeHTTP(blocked, req)
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("blocked origin status = %d, want %d", blocked.Code, http.StatusForbidden)
	}
}

func TestManifestEndpointRejectsOversizedBody(t *testing.T) {
	rec := httptest.NewRecorder()
	body := strings.Repeat(" ", maxRequestBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/v1/manifests/validate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-secret-key")
	newTestHandler(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}
