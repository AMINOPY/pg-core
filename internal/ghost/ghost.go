// Package ghost provides cautious, evidence-based network interference analysis.
// It never treats a single failed probe as proof of DPI and never mutates routes,
// proxy settings, or engine configuration automatically.
package ghost

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AMINOPY/pg-core/internal/diagnostics"
)

const (
	defaultSamples = 5
	maxSamples = 20
	defaultInterval = 200 * time.Millisecond
)

type Finding struct {
	Code       string   `json:"code"`
	Severity   string   `json:"severity"`
	Confidence string   `json:"confidence"`
	Summary    string   `json:"summary"`
	Evidence   []string `json:"evidence"`
}

type Report struct {
	Domain          string                 `json:"domain"`
	Samples         int                    `json:"samples"`
	DNSSuccesses    int                    `json:"dns_successes"`
	HTTPSuccesses   int                    `json:"https_successes"`
	MeanDNSMillis   int64                  `json:"mean_dns_ms,omitempty"`
	MeanHTTPSMillis int64                  `json:"mean_https_ms,omitempty"`
	Findings        []Finding              `json:"findings"`
	Recommendations []string               `json:"recommendations"`
	Observations    []diagnostics.Result   `json:"observations"`
	Disclaimer      string                 `json:"disclaimer"`
}

type Runner struct {
	Checker  diagnostics.Checker
	Samples  int
	Interval time.Duration
}

func (r Runner) sampleCount() int {
	if r.Samples <= 0 { return defaultSamples }
	if r.Samples > maxSamples { return maxSamples }
	return r.Samples
}

func (r Runner) interval() time.Duration {
	if r.Interval <= 0 { return defaultInterval }
	if r.Interval > 5*time.Second { return 5*time.Second }
	return r.Interval
}

// Run performs repeated bounded DNS and HTTPS checks for one explicitly supplied hostname.
func (r Runner) Run(ctx context.Context, rawHost string) (Report, error) {
	host, err := diagnostics.NormalizeDomain(rawHost)
	if err != nil { return Report{}, err }
	count := r.sampleCount()
	report := Report{
		Domain: host, Samples: count,
		Findings: []Finding{},
		Recommendations: []string{},
		Observations: make([]diagnostics.Result, 0, count),
		Disclaimer: "Network symptoms cannot conclusively identify DPI. Results are point-in-time observations; no traffic is intercepted and no network settings are changed.",
	}
	var dnsTotal, httpsTotal time.Duration
	for i := 0; i < count; i++ {
		if err := ctx.Err(); err != nil { return Report{}, err }
		result := r.Checker.Check(ctx, host)
		report.Observations = append(report.Observations, result)
		if result.DNSOK { report.DNSSuccesses++; dnsTotal += result.DNSLatency }
		if result.HTTPSOK { report.HTTPSuccesses++; httpsTotal += result.HTTPSLatency }
		if i+1 < count {
			timer := time.NewTimer(r.interval())
			select {
			case <-ctx.Done():
				if !timer.Stop() { <-timer.C }
				return Report{}, ctx.Err()
			case <-timer.C:
			}
		}
	}
	if report.DNSSuccesses > 0 { report.MeanDNSMillis = (dnsTotal / time.Duration(report.DNSSuccesses)).Milliseconds() }
	if report.HTTPSuccesses > 0 { report.MeanHTTPSMillis = (httpsTotal / time.Duration(report.HTTPSuccesses)).Milliseconds() }
	report.classify()
	return report, nil
}

func (r *Report) classify() {
	n := len(r.Observations)
	if n == 0 { return }
	if r.DNSSuccesses == 0 {
		r.Findings = append(r.Findings, Finding{
			Code: "dns_unreachable", Severity: "high", Confidence: "medium",
			Summary: "DNS resolution failed for every sample; this alone does not distinguish resolver outage, captive portal, local policy, or filtering.",
			Evidence: []string{fmt.Sprintf("DNS succeeded in %d of %d samples", r.DNSSuccesses, n)},
		})
		r.Recommendations = append(r.Recommendations, "Check whether other known-good hostnames resolve and whether the network requires captive-portal sign-in.", "If available, compare with a trusted alternate resolver or another network; do not assume DPI from DNS failure alone.")
		return
	}
	if r.HTTPSuccesses == n {
		r.Findings = append(r.Findings, Finding{
			Code: "https_reachable", Severity: "info", Confidence: "high",
			Summary: "DNS and HTTPS succeeded in all samples; this hostname showed no reachability failure during this test.",
			Evidence: []string{fmt.Sprintf("HTTPS succeeded in %d of %d samples", r.HTTPSuccesses, n)},
		})
		r.Recommendations = append(r.Recommendations, "If the application still fails, test its actual configured endpoint and protocol; a healthy HTTPS hostname does not prove the tunnel data path is healthy.")
		return
	}
	var timeout, tls, reset, policyStatus int
	var statuses []string
	for _, o := range r.Observations {
		if o.HTTPSOK { continue }
		errText := strings.ToLower(o.Error)
		if strings.Contains(errText, "timeout") || strings.Contains(errText, "deadline exceeded") { timeout++ }
		if strings.Contains(errText, "tls") || strings.Contains(errText, "certificate") || strings.Contains(errText, "handshake") { tls++ }
		if strings.Contains(errText, "reset") || strings.Contains(errText, "eof") || strings.Contains(errText, "refused") { reset++ }
		if o.HTTPStatus == 403 || o.HTTPStatus == 451 { policyStatus++ }
		if o.HTTPStatus != 0 { statuses = append(statuses, fmt.Sprintf("HTTP %d", o.HTTPStatus)) }
	}
	if policyStatus > 0 {
		r.Findings = append(r.Findings, Finding{Code:"http_policy_response", Severity:"medium", Confidence:"low", Summary:"At least one HTTP 403/451 response was observed; server policy, a gateway, or filtering may explain it, but DPI is not proven.", Evidence:unique(statuses)})
	}
	if timeout > 0 {
		r.Findings = append(r.Findings, Finding{Code:"https_timeout", Severity:"medium", Confidence:"low", Summary:"HTTPS timed out in one or more samples; congestion, packet loss, server trouble, or filtering can produce the same symptom.", Evidence:[]string{fmt.Sprintf("%d failed samples contained timeout symptoms", timeout)}})
	}
	if tls > 0 {
		r.Findings = append(r.Findings, Finding{Code:"tls_failure", Severity:"medium", Confidence:"low", Summary:"TLS negotiation or certificate errors were observed; clock errors, captive portals, server configuration, and interception are all possible causes.", Evidence:[]string{fmt.Sprintf("%d failed samples contained TLS/certificate symptoms", tls)}})
	}
	if reset > 0 {
		r.Findings = append(r.Findings, Finding{Code:"connection_interrupted", Severity:"medium", Confidence:"low", Summary:"Connection reset/EOF/refusal symptoms were observed; these do not uniquely identify DPI.", Evidence:[]string{fmt.Sprintf("%d failed samples contained reset/EOF/refusal symptoms", reset)}})
	}
	if len(r.Findings) == 0 {
		r.Findings = append(r.Findings, Finding{Code:"intermittent_https_failure", Severity:"medium", Confidence:"low", Summary:"HTTPS failed in some samples but the error pattern is inconclusive.", Evidence:[]string{fmt.Sprintf("HTTPS succeeded in %d of %d samples", r.HTTPSuccesses, n)}})
	}
	r.Recommendations = append(r.Recommendations,
		"Compare the same check on a trusted alternate network and against a second known-good HTTPS host to separate host outage from network-specific failure.",
		"Check system time, captive-portal status, DNS settings, and the target server's status before attributing symptoms to DPI.",
		"If you administer the connection, test only an already configured, supported alternate transport/profile and verify end-to-end connectivity before switching back; Ghost does not invent credentials or automatically change routes.",
	)
}

func unique(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values { if v != "" && !seen[v] { seen[v] = true; out = append(out, v) } }
	if len(out) == 0 { return []string{"HTTP failure observed; inspect the detailed observations"} }
	return out
}

var ErrNoEvidence = errors.New("no diagnostic observations")
