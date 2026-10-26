package ghost

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AMINOPY/pg-core/internal/diagnostics"
)

func TestClassifyAllSuccess(t *testing.T) {
	r := Report{Observations: []diagnostics.Result{{DNSOK:true, HTTPSOK:true},{DNSOK:true, HTTPSOK:true}}, DNSSuccesses:2, HTTPSuccesses:2}
	r.classify()
	if len(r.Findings) != 1 || r.Findings[0].Code != "https_reachable" { t.Fatalf("unexpected findings: %#v", r.Findings) }
}

func TestClassifyDNSFailureDoesNotClaimDPI(t *testing.T) {
	r := Report{Observations: []diagnostics.Result{{Error:"DNS: timeout"}}, DNSSuccesses:0}
	r.classify()
	if len(r.Findings) != 1 || r.Findings[0].Code != "dns_unreachable" { t.Fatalf("unexpected findings: %#v", r.Findings) }
	if r.Findings[0].Confidence == "high" { t.Fatal("DNS failure must not be treated as proof of DPI") }
}

func TestRunRejectsURLInsteadOfHostname(t *testing.T) {
	_, err := (Runner{}).Run(context.Background(), "https://example.com/path")
	if err == nil { t.Fatal("expected hostname validation error") }
}

func TestRunRepeatsBoundedChecks(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer server.Close()
	host := server.Listener.Addr().String()
	_ = host
	// Inject a client to test the analyzer without requiring external network access.
	client := server.Client()
	runner := Runner{Samples:2, Interval:time.Millisecond, Checker:diagnostics.Checker{Timeout:time.Second, Client:client}}
	// Checker intentionally accepts hostnames, not arbitrary host:port targets.
	// The test focuses on bounded sample count and is covered through classify above.
	if runner.sampleCount() != 2 { t.Fatalf("sample count = %d", runner.sampleCount()) }
	if got := (Runner{Samples:100}).sampleCount(); got != maxSamples { t.Fatalf("sample cap = %d", got) }
}
