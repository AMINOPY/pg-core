package ai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSpeedTesterMeasuresConfiguredEndpoints(t *testing.T) {
	download := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("download method = %s", r.Method)
		}
		_, _ = io.WriteString(w, strings.Repeat("d", 4096))
	}))
	defer download.Close()

	upload := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("upload method = %s", r.Method)
		}
		n, err := io.Copy(io.Discard, r.Body)
		if err != nil {
			t.Errorf("read upload: %v", err)
		}
		if n != 1024 {
			t.Errorf("upload bytes = %d, want 1024", n)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upload.Close()

	report := (SpeedTester{Timeout: time.Second, MaxDownloadBytes: 2048, MaxUploadBytes: 1024}).Run(context.Background(), download.URL, upload.URL)
	if report.DownloadError != "" {
		t.Fatalf("download error: %s", report.DownloadError)
	}
	if report.DownloadBytes != 2048 || report.DownloadMbps <= 0 {
		t.Fatalf("unexpected download report: %+v", report)
	}
	if report.UploadError != "" {
		t.Fatalf("upload error: %s", report.UploadError)
	}
	if report.UploadBytes != 1024 || report.UploadMbps <= 0 {
		t.Fatalf("unexpected upload report: %+v", report)
	}
}

func TestSpeedTesterRejectsInsecureRemoteEndpoint(t *testing.T) {
	report := (SpeedTester{}).Run(context.Background(), "http://example.com/download", "")
	if report.DownloadError == "" {
		t.Fatal("expected insecure remote endpoint to be rejected")
	}
}

func TestSpeedTesterRejectsCredentials(t *testing.T) {
	report := (SpeedTester{}).Run(context.Background(), "https://user:pass@example.com/download", "")
	if report.DownloadError == "" {
		t.Fatal("expected URL credentials to be rejected")
	}
}

func TestSpeedTesterDoesNotExposeURLPathOrQueryInRequestErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rawURL := server.URL + "/private/path?token=secret-value"
	server.Close()

	report := (SpeedTester{Timeout: time.Second}).Run(context.Background(), rawURL, "")
	if report.DownloadError == "" {
		t.Fatal("expected request failure")
	}
	if strings.Contains(report.DownloadError, "/private/path") || strings.Contains(report.DownloadError, "secret-value") {
		t.Fatalf("request error leaked URL details: %q", report.DownloadError)
	}
}

func TestSpeedTesterBoundsPayload(t *testing.T) {
	s := SpeedTester{MaxDownloadBytes: 100 << 20, MaxUploadBytes: 100 << 20}
	if got := s.downloadLimit(); got != 50<<20 {
		t.Fatalf("download limit = %d", got)
	}
	if got := s.uploadLimit(); got != 10<<20 {
		t.Fatalf("upload limit = %d", got)
	}
}
