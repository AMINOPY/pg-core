package quickconnect

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewFromEnvRequiresServerKey(t *testing.T) {
	old, had := os.LookupEnv("CHIMERA_TUNNEL_KEY")
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("CHIMERA_TUNNEL_KEY", old)
		} else {
			_ = os.Unsetenv("CHIMERA_TUNNEL_KEY")
		}
	})
	_ = os.Unsetenv("CHIMERA_TUNNEL_KEY")
	if _, err := NewFromEnv(); err == nil {
		t.Fatal("expected missing key to be rejected")
	}
	_ = os.Setenv("CHIMERA_TUNNEL_KEY", "short")
	if _, err := NewFromEnv(); err == nil {
		t.Fatal("expected short key to be rejected")
	}
	_ = os.Setenv("CHIMERA_TUNNEL_KEY", "a-secure-example-key-at-least-24")
	if _, err := NewFromEnv(); err != nil {
		t.Fatalf("valid key rejected: %v", err)
	}
}

func TestHandlerRejectsNonLoopbackHost(t *testing.T) {
	s := &Server{key: "a-secure-example-key-at-least-24"}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestConnectRejectsInvalidInputBeforeStarting(t *testing.T) {
	s := &Server{key: "a-secure-example-key-at-least-24"}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8787/api/connect", strings.NewReader(`{"sni":"not a host","edge_ips":"not-an-ip"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestConnectRejectsTrailingJSONValues(t *testing.T) {
	s := &Server{key: "a-secure-example-key-at-least-24"}
	body := `{"sni":"valid.example.com","edge_ips":"1.1.1.1"} {"extra":true}`
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8787/api/connect", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestAutoSelectRejectsTrailingJSONValues(t *testing.T) {
	s := &Server{key: "a-secure-example-key-at-least-24"}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8787/api/auto/select", strings.NewReader(`{"mode":"auto"} {}`))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestAutoSelectReportsMissingCatalogWithoutBreakingManualUI(t *testing.T) {
	s := &Server{key: "a-secure-example-key-at-least-24"}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8787/api/auto/select", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Auto mode is not configured") {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}

func TestAutoSelectRejectsUnknownFields(t *testing.T) {
	s := &Server{key: "a-secure-example-key-at-least-24"}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8787/api/auto/select", strings.NewReader(`{"mode":"auto","unexpected":true}`))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestAutoSelectRejectsEmptyBody(t *testing.T) {
	s := &Server{key: "a-secure-example-key-at-least-24"}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8787/api/auto/select", strings.NewReader(""))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestAutoSelectRejectsUnknownMode(t *testing.T) {
	s := &Server{key: "a-secure-example-key-at-least-24"}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8787/api/auto/select", strings.NewReader(`{"mode":"unknown"}`))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestRegionalModesReportMissingCatalog(t *testing.T) {
	for _, tc := range []struct {
		mode string
		want string
	}{
		{mode: "ir", want: "CDN IR is not configured"},
		{mode: "us", want: "CDN US is not configured"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			s := &Server{key: "a-secure-example-key-at-least-24"}
			body := strings.NewReader(`{"mode":"` + tc.mode + `"}`)
			req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8787/api/auto/select", body)
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("response %q does not contain %q", rec.Body.String(), tc.want)
			}
		})
	}
}
