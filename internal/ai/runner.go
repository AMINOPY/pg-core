package ai

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/AMINOPY/pg-core/internal/diagnostics"
)

type Runner struct {
	Checker  diagnostics.Checker
	Samples  int
	Interval time.Duration

	// Throughput tests are opt-in and run only against these explicitly supplied URLs.
	DownloadURL string
	UploadURL   string
	SpeedTester SpeedTester
}

func (r Runner) sampleCount() int {
	if r.Samples <= 0 {
		return 5
	}
	if r.Samples > 20 {
		return 20
	}
	return r.Samples
}

func (r Runner) interval() time.Duration {
	if r.Interval <= 0 {
		return 150 * time.Millisecond
	}
	if r.Interval > 5*time.Second {
		return 5 * time.Second
	}
	return r.Interval
}

// Run performs bounded DNS/HTTPS checks and optional configured throughput tests.
// It only contacts the explicitly supplied hostname and speed-test endpoints.
func (r Runner) Run(ctx context.Context, hostname string) (Report, error) {
	host, err := diagnostics.NormalizeDomain(hostname)
	if err != nil {
		return Report{}, err
	}
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}

	count := r.sampleCount()
	report := Report{
		GeneratedAt: time.Now().UTC(),
		Target:      host,
		Samples:     count,
		Checks:      make([]diagnostics.Result, 0, count),
		Findings:    []Finding{},
		Disclaimer: "Point-in-time observations. HTTPS timings are not ICMP ping. Throughput is measured only when configured endpoints are supplied; carrier and SIM details are not inferred.",
	}
	checker := r.Checker
	if checker.Timeout <= 0 {
		checker.Timeout = 4 * time.Second
	}
	if checker.Workers <= 0 {
		checker.Workers = 1
	}
	var total time.Duration
	var previous time.Duration
	var jitterTotal time.Duration
	var jitterPairs int
	for i := 0; i < count; i++ {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		result := checker.Check(ctx, host)
		report.Checks = append(report.Checks, result)
		if result.DNSOK {
			report.DNSOK = true
		}
		if result.HTTPSOK {
			report.HTTPSOK = true
			latency := result.HTTPSLatency
			if report.Successful == 0 || latency < report.MinLatency {
				report.MinLatency = latency
			}
			if report.Successful == 0 || latency > report.MaxLatency {
				report.MaxLatency = latency
			}
			report.Successful++
			total += latency
			if report.Successful > 1 {
				delta := latency - previous
				if delta < 0 {
					delta = -delta
				}
				jitterTotal += delta
				jitterPairs++
			}
			previous = latency
		}
		if i+1 < count {
			timer := time.NewTimer(r.interval())
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return report, ctx.Err()
			case <-timer.C:
			}
		}
	}
	report.PacketLoss = float64(count-report.Successful) * 100 / float64(count)
	if report.Successful > 0 {
		report.AvgLatency = total / time.Duration(report.Successful)
	}
	if jitterPairs > 0 {
		report.Jitter = jitterTotal / time.Duration(jitterPairs)
	}

	if r.DownloadURL != "" || r.UploadURL != "" {
		speedReport := r.SpeedTester.Run(ctx, r.DownloadURL, r.UploadURL)
		report.SpeedTest = &speedReport
	}
	report.Findings = analyze(report)
	return report, nil
}

func analyze(r Report) []Finding {
	findings := make([]Finding, 0, 6)
	switch {
	case !r.DNSOK:
		findings = append(findings, Finding{
			Code:        "DNS_UNAVAILABLE",
			Severity:    SeverityError,
			Title:       "DNS lookup failed",
			Explanation: "The system resolver did not return an address for the target.",
			Evidence:    "No successful DNS sample was recorded.",
		})
	case !r.HTTPSOK:
		findings = append(findings, Finding{
			Code:        "HTTPS_UNREACHABLE",
			Severity:    SeverityError,
			Title:       "HTTPS check failed",
			Explanation: "DNS worked, but no HTTPS sample completed successfully.",
			Evidence:    fmt.Sprintf("%d of %d HTTPS samples succeeded.", r.Successful, r.Samples),
		})
	}
	if r.Successful > 0 {
		switch {
		case r.PacketLoss >= 40:
			findings = append(findings, Finding{
				Code:        "CONNECTION_UNSTABLE",
				Severity:    SeverityError,
				Title:       "Connection appears unstable",
				Explanation: "Many HTTPS probes failed during this short test. Retry and compare another known endpoint before drawing conclusions.",
				Evidence:    fmt.Sprintf("%.0f%% failed samples.", r.PacketLoss),
			})
		case r.PacketLoss > 0:
			findings = append(findings, Finding{
				Code:        "SOME_PROBES_FAILED",
				Severity:    SeverityWarn,
				Title:       "Some probes failed",
				Explanation: "A few requests failed; transient congestion, server behavior, or local connectivity may explain this.",
				Evidence:    fmt.Sprintf("%.0f%% failed samples.", r.PacketLoss),
			})
		}
		if r.AvgLatency > 800*time.Millisecond {
			findings = append(findings, Finding{
				Code:        "HIGH_LATENCY",
				Severity:    SeverityWarn,
				Title:       "High response latency",
				Explanation: "Average HTTPS response time was high for this test. This includes network and server processing time, not pure ICMP ping.",
				Evidence:    fmt.Sprintf("Average %.0f ms; minimum %.0f ms.", r.AvgLatency.Seconds()*1000, r.MinLatency.Seconds()*1000),
			})
		}
		if r.Jitter > 250*time.Millisecond {
			findings = append(findings, Finding{
				Code:        "HIGH_VARIATION",
				Severity:    SeverityWarn,
				Title:       "Response time varies",
				Explanation: "Successful request timings changed significantly between samples.",
				Evidence:    fmt.Sprintf("Mean absolute change %.0f ms.", r.Jitter.Seconds()*1000),
			})
		}
	}
	if r.SpeedTest != nil {
		if r.SpeedTest.DownloadError != "" {
			findings = append(findings, Finding{
				Code:        "DOWNLOAD_TEST_FAILED",
				Severity:    SeverityWarn,
				Title:       "Download test failed",
				Explanation: "The configured download endpoint could not complete the measurement.",
				Evidence:    r.SpeedTest.DownloadError,
			})
		}
		if r.SpeedTest.UploadError != "" {
			findings = append(findings, Finding{
				Code:        "UPLOAD_TEST_FAILED",
				Severity:    SeverityWarn,
				Title:       "Upload test failed",
				Explanation: "The configured upload endpoint could not complete the measurement.",
				Evidence:    r.SpeedTest.UploadError,
			})
		}
	}
	if len(findings) == 0 {
		evidence := fmt.Sprintf("%d of %d HTTPS samples succeeded.", r.Successful, r.Samples)
		if r.SpeedTest != nil && r.SpeedTest.DownloadMbps > 0 && r.SpeedTest.UploadMbps > 0 {
			evidence = fmt.Sprintf("%s Download %.2f Mbps; upload %.2f Mbps.", evidence, r.SpeedTest.DownloadMbps, r.SpeedTest.UploadMbps)
		}
		findings = append(findings, Finding{
			Code:        "NO_MAJOR_ISSUE_OBSERVED",
			Severity:    SeverityInfo,
			Title:       "No major issue observed",
			Explanation: "These limited checks did not reveal a major problem. Results describe only this endpoint and test window.",
			Evidence:    evidence,
		})
	}
	return findings
}

var ErrNoSamples = errors.New("no diagnostic samples were collected")
