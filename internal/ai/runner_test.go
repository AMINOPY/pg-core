package ai

import (
	"context"
	"testing"
	"time"
)

func TestSampleCountIsBounded(t *testing.T) {
	if got := (Runner{Samples: 100}).sampleCount(); got != 20 { t.Fatalf("got %d", got) }
	if got := (Runner{}).sampleCount(); got != 5 { t.Fatalf("got %d", got) }
}

func TestInvalidTargetRejectedWithoutNetwork(t *testing.T) {
	_, err := (Runner{}).Run(context.Background(), "https://example.com/path")
	if err == nil { t.Fatal("expected hostname validation error") }
}

func TestCancellationHonored(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (Runner{Interval: time.Millisecond}).Run(ctx, "example.com")
	if err == nil { t.Fatal("expected cancellation") }
}

func TestAnalyzeReportsFailures(t *testing.T) {
	findings := analyze(Report{Target:"example.com", Samples:5, Successful:1, PacketLoss:80, DNSOK:true, HTTPSOK:true})
	found := false
	for _, f := range findings { if f.Code == "CONNECTION_UNSTABLE" { found = true } }
	if !found { t.Fatal("expected instability finding") }
}
