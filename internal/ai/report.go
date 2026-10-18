// Package ai provides explainable, bounded network diagnostics for the AI feature.
package ai

import (
	"time"

	"github.com/AMINOPY/pg-core/internal/diagnostics"
)

type Severity string

const (
	SeverityInfo Severity = "info"
	SeverityWarn Severity = "warning"
	SeverityError Severity = "error"
)

type Finding struct {
	Code        string   `json:"code"`
	Severity    Severity `json:"severity"`
	Title       string   `json:"title"`
	Explanation string   `json:"explanation"`
	Evidence    string   `json:"evidence"`
}

type Report struct {
	GeneratedAt time.Time            `json:"generated_at"`
	Target      string               `json:"target"`
	Samples     int                  `json:"samples"`
	Successful  int                  `json:"successful_samples"`
	PacketLoss  float64              `json:"failed_sample_percent"`
	MinLatency  time.Duration        `json:"min_https_latency_ns"`
	AvgLatency  time.Duration        `json:"avg_https_latency_ns"`
	MaxLatency  time.Duration        `json:"max_https_latency_ns"`
	Jitter      time.Duration        `json:"jitter_ns"`
	DNSOK       bool                 `json:"dns_ok"`
	HTTPSOK     bool                 `json:"https_ok"`
	Checks      []diagnostics.Result `json:"checks"`
	SpeedTest   *SpeedTestReport     `json:"speed_test,omitempty"`
	Findings    []Finding            `json:"findings"`
	Disclaimer  string               `json:"disclaimer"`
}
