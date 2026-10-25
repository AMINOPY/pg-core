package ai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultMaxDownloadBytes int64 = 10 << 20
	defaultMaxUploadBytes   int64 = 1 << 20
	maxResponseBytes        int64 = 64 << 10
)

// SpeedTestReport contains opt-in throughput measurements against endpoints
// explicitly configured by the operator. Mbps is decimal megabits per second.
type SpeedTestReport struct {
	DownloadHost     string        `json:"download_host,omitempty"`
	UploadHost       string        `json:"upload_host,omitempty"`
	DownloadBytes    int64         `json:"download_bytes,omitempty"`
	UploadBytes      int64         `json:"upload_bytes,omitempty"`
	DownloadDuration time.Duration `json:"download_duration_ns,omitempty"`
	UploadDuration   time.Duration `json:"upload_duration_ns,omitempty"`
	DownloadMbps     float64       `json:"download_mbps,omitempty"`
	UploadMbps       float64       `json:"upload_mbps,omitempty"`
	DownloadError    string        `json:"download_error,omitempty"`
	UploadError      string        `json:"upload_error,omitempty"`
}

// SpeedTester performs bounded HTTP transfer measurements. It never selects
// a third-party server automatically: callers must supply trusted endpoints.
type SpeedTester struct {
	Client           *http.Client
	Timeout          time.Duration
	MaxDownloadBytes int64
	MaxUploadBytes   int64
}

func (s SpeedTester) timeout() time.Duration {
	if s.Timeout <= 0 || s.Timeout > 45*time.Second {
		return 15 * time.Second
	}
	return s.Timeout
}

func (s SpeedTester) downloadLimit() int64 {
	if s.MaxDownloadBytes <= 0 {
		return defaultMaxDownloadBytes
	}
	if s.MaxDownloadBytes > 50<<20 {
		return 50 << 20
	}
	return s.MaxDownloadBytes
}

func (s SpeedTester) uploadLimit() int64 {
	if s.MaxUploadBytes <= 0 {
		return defaultMaxUploadBytes
	}
	if s.MaxUploadBytes > 10<<20 {
		return 10 << 20
	}
	return s.MaxUploadBytes
}

func validateSpeedURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid speed-test URL: %w", err)
	}
	if u.Host == "" || u.User != nil || u.Fragment != "" {
		return nil, errors.New("speed-test URL must have a host and must not contain credentials or a fragment")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && isLoopbackHost(u.Hostname())) {
		return nil, errors.New("speed-test URL must use HTTPS (HTTP is allowed only for localhost tests)")
	}
	return u, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s SpeedTester) client() *http.Client {
	var c http.Client
	if s.Client != nil {
		c = *s.Client
	}
	if c.Timeout == 0 || c.Timeout > s.timeout() {
		c.Timeout = s.timeout()
	}
	// Do not forward a POST payload to a redirected host.
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &c
}

// Run executes whichever measurements have URLs configured. Both URLs may be
// omitted, but a configured URL must pass validation before any request occurs.
func (s SpeedTester) Run(ctx context.Context, downloadURL, uploadURL string) SpeedTestReport {
	out := SpeedTestReport{}
	if strings.TrimSpace(downloadURL) != "" {
		u, err := validateSpeedURL(downloadURL)
		if err != nil {
			out.DownloadError = err.Error()
		} else {
			out.DownloadHost = u.Hostname()
			out.DownloadBytes, out.DownloadDuration, out.DownloadMbps, out.DownloadError = s.download(ctx, u)
		}
	}
	if strings.TrimSpace(uploadURL) != "" {
		u, err := validateSpeedURL(uploadURL)
		if err != nil {
			out.UploadError = err.Error()
		} else {
			out.UploadHost = u.Hostname()
			out.UploadBytes, out.UploadDuration, out.UploadMbps, out.UploadError = s.upload(ctx, u)
		}
	}
	return out
}

func (s SpeedTester) download(ctx context.Context, u *url.URL) (int64, time.Duration, float64, string) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, 0, 0, err.Error()
	}
	start := time.Now()
	resp, err := s.client().Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return 0, elapsed, 0, err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, elapsed, 0, fmt.Sprintf("download endpoint returned HTTP %d", resp.StatusCode)
	}
	n, readErr := io.Copy(io.Discard, io.LimitReader(resp.Body, s.downloadLimit()))
	elapsed = time.Since(start)
	if readErr != nil {
		return n, elapsed, 0, readErr.Error()
	}
	if elapsed <= 0 {
		elapsed = time.Nanosecond
	}
	return n, elapsed, mbps(n, elapsed), ""
}

func (s SpeedTester) upload(ctx context.Context, u *url.URL) (int64, time.Duration, float64, string) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout())
	defer cancel()

	payload := bytes.Repeat([]byte("c"), int(s.uploadLimit()))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(payload))
	if err != nil {
		return 0, 0, 0, err.Error()
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Length", fmt.Sprint(len(payload)))
	start := time.Now()
	resp, err := s.client().Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return 0, elapsed, 0, err.Error()
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
	elapsed = time.Since(start)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return int64(len(payload)), elapsed, 0, fmt.Sprintf("upload endpoint returned HTTP %d", resp.StatusCode)
	}
	if elapsed <= 0 {
		elapsed = time.Nanosecond
	}
	return int64(len(payload)), elapsed, mbps(int64(len(payload)), elapsed), ""
}

func mbps(n int64, elapsed time.Duration) float64 {
	if n <= 0 || elapsed <= 0 {
		return 0
	}
	return float64(n) * 8 / elapsed.Seconds() / 1_000_000
}
