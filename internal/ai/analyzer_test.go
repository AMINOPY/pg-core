package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAnalyzeJSONProducesActionableFindings(t *testing.T) {
	raw := `{"network_type":"cellular","cellular_generation":"4g","download_mbps":3,"upload_mbps":0.2,"latency_ms":180,"jitter_ms":35,"packet_loss_pct":3,"dns_latency_ms":300,"checks":[{"name":"chosen-endpoint","reachable":false,"dns_latency_ms":20,"https_latency_ms":0}]}`
	out, err := AnalyzeJSON(raw)
	if err != nil { t.Fatal(err) }
	var result Result
	if err := json.Unmarshal([]byte(out), &result); err != nil { t.Fatal(err) }
	codes := map[string]bool{}
	for _, f := range result.Findings { codes[f.Code] = true }
	for _, code := range []string{"high_latency","high_jitter","packet_loss","low_download","low_upload","slow_dns","endpoint_unreachable"} {
		if !codes[code] { t.Errorf("missing finding %q", code) }
	}
	if result.Score >= 70 { t.Fatalf("expected low score, got %d", result.Score) }
}

func TestAnalyzeJSONRejectsMalformedInput(t *testing.T) {
	for _, raw := range []string{
		`{"network_type":"wifi","download_mbps":1,"upload_mbps":1,"latency_ms":1,"jitter_ms":0,"packet_loss_pct":0,"dns_latency_ms":1,"extra":true}`,
		`{"network_type":"other","download_mbps":1,"upload_mbps":1,"latency_ms":1,"jitter_ms":0,"packet_loss_pct":0,"dns_latency_ms":1}`,
		`{"network_type":"wifi","download_mbps":-1,"upload_mbps":1,"latency_ms":1,"jitter_ms":0,"packet_loss_pct":0,"dns_latency_ms":1}`,
		strings.Repeat("x", MaxReportBytes+1),
	} {
		if _, err := AnalyzeJSON(raw); err == nil { t.Errorf("accepted invalid input") }
	}
}

func TestAnalyzeWorksWithoutSIMMetadata(t *testing.T) {
	result, err := Analyze(Report{NetworkType:"wifi", DownloadMbps:100, UploadMbps:20, LatencyMS:15, JitterMS:2, DNSLatencyMS:20})
	if err != nil { t.Fatal(err) }
	if result.Score < 85 { t.Fatalf("unexpected score: %d", result.Score) }
}
