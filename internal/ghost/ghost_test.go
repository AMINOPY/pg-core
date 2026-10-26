package ghost

import (
	"context"
	"testing"

	"github.com/AMINOPY/pg-core/internal/diagnostics"
)

func TestClassifyAllSuccess(t *testing.T) {
	report := Report{
		Observations:	[]diagnostics.Result{{DNSOK: true, HTTPSOK: true}, {DNSOK: true, HTTPSOK: true}},
		DNSSuccesses:	2,
		HTTPSuccesses:	2,
	}
	report.classify()
	if len(report.Findings) != 1 || report.Findings[0].Code != "https_reachable" {
		t.Fatalf("unexpected findings: %#v", report.Findings)
	}
}

func TestClassifyDNSFailureDoesNotClaimDPI(t *testing.T) {
	report := Report{
		Observations: []diagnostics.Result{{Error: "DNS: timeout"}},
		DNSSuccesses: 0,
	}
	report.classify()
	if len(report.Findings) != 1 || report.Findings[0].Code != "dns_unreachable" {
		t.Fatalf("unexpected findings: %#v", report.Findings)
	}
	if report.Findings[0].Confidence == "high" {
		t.Fatal("DNS failure must not be treated as proof of DPI")
	}
}

func TestClassifyTLSAndResetSymptoms(t *testing.T) {
	report := Report{
		Observations: []diagnostics.Result{
			{DNSOK: true, Error: "HTTPS: TLS handshake timeout"},
			{DNSOK: true, Error: "HTTPS: read: connection reset by peer"},
		},
		DNSSuccesses: 2,
	}
	report.classify()
	codes := map[string]bool{}
	for _, finding := range report.Findings {
		codes[finding.Code] = true
		if finding.Confidence == "high" {
			t.Fatalf("ambiguous symptoms cannot have high confidence: %#v", finding)
		}
	}
	if !codes["https_timeout"] || !codes["tls_failure"] || !codes["connection_interrupted"] {
		t.Fatalf("expected timeout/TLS/reset findings, got %#v", report.Findings)
	}
}

func TestRunRejectsURLInsteadOfHostname(t *testing.T) {
	_, err := (Runner{}).Run(context.Background(), "https://example.com/path")
	if err == nil {
		t.Fatal("expected hostname validation error")
	}
}

func TestSampleAndIntervalBounds(t *testing.T) {
	if got := (Runner{}).sampleCount(); got != defaultSamples {
		t.Fatalf("default sample count = %d", got)
	}
	if got := (Runner{Samples: 100}).sampleCount(); got != maxSamples {
		t.Fatalf("sample cap = %d", got)
	}
	if got := (Runner{Interval: 10_000_000_000}).interval(); got != 5_000_000_000 {
		t.Fatalf("interval cap = %s", got)
	}
}
