// Package ai provides explainable, privacy-preserving network diagnostics.
// It uses deterministic heuristics; it does not call a remote AI service.
package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"unicode"
)

const (
	MaxReportBytes = 64 << 10
	MaxChecks      = 64
	MaxTextLength  = 128
)

var ErrInvalidReport = errors.New("invalid network report")

type Check struct {
	Name         string  `json:"name"`
	Reachable    bool    `json:"reachable"`
	DNSLatencyMS float64 `json:"dns_latency_ms"`
	HTTPSLatencyMS float64 `json:"https_latency_ms"`
}

type Report struct {
	NetworkType       string  `json:"network_type"`
	CellularGeneration string `json:"cellular_generation,omitempty"`
	Carrier           string  `json:"carrier,omitempty"`
	SimState          string  `json:"sim_state,omitempty"`
	Roaming           bool    `json:"roaming,omitempty"`
	DownloadMbps      float64 `json:"download_mbps"`
	UploadMbps        float64 `json:"upload_mbps"`
	LatencyMS         float64 `json:"latency_ms"`
	JitterMS          float64 `json:"jitter_ms"`
	PacketLossPct     float64 `json:"packet_loss_pct"`
	DNSLatencyMS      float64 `json:"dns_latency_ms"`
	SignalStrengthDBM int     `json:"signal_strength_dbm,omitempty"`
	Checks            []Check `json:"checks,omitempty"`
}

type Finding struct {
	Code       string  `json:"code"`
	Severity   string  `json:"severity"`
	Confidence float64 `json:"confidence"`
	Summary    string  `json:"summary"`
	Advice     string  `json:"advice"`
}

type Result struct {
	SchemaVersion int       `json:"schema_version"`
	Score         int       `json:"network_score"`
	Grade         string    `json:"grade"`
	Findings      []Finding `json:"findings"`
	Limitations   []string  `json:"limitations"`
}

func AnalyzeJSON(raw string) (string, error) {
	if len(raw) == 0 || len(raw) > MaxReportBytes {
		return "", ErrInvalidReport
	}
	var report Report
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return "", fmt.Errorf("%w: malformed JSON", ErrInvalidReport)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return "", ErrInvalidReport
	}
	result, err := Analyze(report)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func Analyze(r Report) (Result, error) {
	if !oneOf(r.NetworkType, "wifi", "cellular", "ethernet", "unknown") ||
		!optionalOneOf(r.CellularGeneration, "2g", "3g", "4g", "5g", "unknown") ||
		!optionalOneOf(r.SimState, "ready", "absent", "locked", "unknown") ||
		!validText(r.Carrier, MaxTextLength) ||
		!finiteRange(r.DownloadMbps, 0, 100000) ||
		!finiteRange(r.UploadMbps, 0, 100000) ||
		!finiteRange(r.LatencyMS, 0, 60000) ||
		!finiteRange(r.JitterMS, 0, 60000) ||
		!finiteRange(r.PacketLossPct, 0, 100) ||
		!finiteRange(r.DNSLatencyMS, 0, 60000) ||
		(r.SignalStrengthDBM != 0 && (r.SignalStrengthDBM < -160 || r.SignalStrengthDBM > 0)) ||
		len(r.Checks) > MaxChecks {
		return Result{}, ErrInvalidReport
	}
	for _, check := range r.Checks {
		if !validText(check.Name, MaxTextLength) || strings.TrimSpace(check.Name) == "" ||
			!finiteRange(check.DNSLatencyMS, 0, 60000) ||
			!finiteRange(check.HTTPSLatencyMS, 0, 60000) {
			return Result{}, ErrInvalidReport
		}
	}
	score := 100
	if r.LatencyMS > 50 { score -= minInt(20, int((r.LatencyMS-50)/10)) }
	if r.JitterMS > 10 { score -= minInt(15, int((r.JitterMS-10)/5)) }
	if r.PacketLossPct > 0 { score -= minInt(35, int(math.Ceil(r.PacketLossPct*3))) }
	if r.DownloadMbps < 25 { score -= minInt(20, int((25-r.DownloadMbps)*0.8)) }
	if r.UploadMbps < 5 { score -= minInt(10, int((5-r.UploadMbps)*2)) }
	if r.DNSLatencyMS > 100 { score -= minInt(15, int((r.DNSLatencyMS-100)/20)) }
	if score < 0 { score = 0 }

	findings := make([]Finding, 0, 8)
	add := func(code, severity string, confidence float64, summary, advice string) {
		findings = append(findings, Finding{Code:code, Severity:severity, Confidence:confidence, Summary:summary, Advice:advice})
	}
	if r.LatencyMS >= 150 {
		add("high_latency", "warning", 0.92, "Latency is high for interactive use.", "Compare a nearby test server and repeat when the network is idle.")
	} else if r.LatencyMS >= 70 {
		add("moderate_latency", "info", 0.82, "Latency may affect real-time apps.", "Compare results on Wi-Fi and cellular, if available.")
	}
	if r.JitterMS >= 30 {
		add("high_jitter", "warning", 0.9, "Latency varies substantially between samples.", "Repeat the test and check whether another app is saturating the connection.")
	}
	if r.PacketLossPct >= 2 {
		add("packet_loss", "warning", 0.94, "Packet loss is high enough to disrupt calls and interactive sessions.", "Repeat against a second trusted endpoint; if persistent, check signal quality or router stability.")
	} else if r.PacketLossPct > 0 {
		add("minor_packet_loss", "info", 0.7, "Some packet loss was measured.", "Repeat the test because small samples can exaggerate loss.")
	}
	if r.DownloadMbps < 5 {
		add("low_download", "warning", 0.88, "Measured download throughput is low.", "Repeat with a nearby test server and pause other large downloads.")
	}
	if r.UploadMbps < 1 {
		add("low_upload", "warning", 0.86, "Measured upload throughput is low.", "Check background uploads and repeat with a trusted test server.")
	}
	if r.DNSLatencyMS >= 250 {
		add("slow_dns", "warning", 0.84, "DNS lookups are slow.", "Compare the system resolver with a trusted resolver permitted by your network or organization.")
	}
	unreachable := 0
	for _, c := range r.Checks { if !c.Reachable { unreachable++ } }
	if len(r.Checks) > 0 && unreachable > 0 {
		add("endpoint_unreachable", "info", 0.72, fmt.Sprintf("%d of %d explicitly selected endpoints did not respond.", unreachable, len(r.Checks)), "Retry later and compare another endpoint; one failed check does not prove a service is universally blocked.")
	}
	if r.NetworkType == "cellular" && r.CellularGeneration != "" && r.CellularGeneration != "unknown" {
		if r.CellularGeneration == "2g" || r.CellularGeneration == "3g" {
			add("older_radio_generation", "info", 0.95, "The device reports an older cellular radio generation.", "If your plan and coverage support it, check preferred network mode and local coverage.")
		}
	}
	if r.SignalStrengthDBM != 0 && r.SignalStrengthDBM < -115 {
		add("weak_signal", "warning", 0.82, "Reported cellular signal strength is weak.", "Move to a location with better coverage and repeat the test.")
	}
	if r.Roaming {
		add("roaming", "info", 0.98, "The device reports that cellular roaming is active.", "Check your plan's roaming terms before running large speed tests.")
	}
	if len(findings) == 0 {
		add("no_major_issue_detected", "info", 0.65, "No major issue was detected from the supplied measurements.", "Run more than one test at different times; this is not a guarantee of service quality.")
	}
	sort.SliceStable(findings, func(i, j int) bool {
		rank := map[string]int{"critical":0, "warning":1, "info":2}
		return rank[findings[i].Severity] < rank[findings[j].Severity]
	})
	grade := "poor"
	if score >= 85 { grade = "excellent" } else if score >= 70 { grade = "good" } else if score >= 45 { grade = "fair" }
	return Result{SchemaVersion:1, Score:score, Grade:grade, Findings:findings, Limitations:[]string{
		"Analysis is deterministic and rule-based; it is not a trained AI model or a carrier diagnosis.",
		"Speed-test results depend on the chosen server, device load, test duration, and network conditions.",
		"SIM/carrier metadata is optional and does not by itself determine internet speed.",
		"Only user-supplied endpoint checks are summarized; no IP-range scanning is performed.",
	}}, nil
}

func validText(s string, max int) bool {
	if len(s) > max { return false }
	for _, r := range s { if unicode.IsControl(r) { return false } }
	return true
}
func finiteRange(v, min, max float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= min && v <= max }
func oneOf(v string, values ...string) bool { for _, x := range values { if v == x { return true } }; return false }
func optionalOneOf(v string, values ...string) bool { return v == "" || oneOf(v, values...) }
func minInt(a, b int) int { if a < b { return a }; return b }
