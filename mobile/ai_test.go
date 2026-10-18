package mobile

import (
	"encoding/json"
	"testing"
)

func TestAnalyzeNetworkJSONReturnsScoreAndFindings(t *testing.T) {
	raw, err := AnalyzeNetworkJSON(`{"network_type":"cellular","cellular_generation":"4g","download_mbps":3,"upload_mbps":0.2,"latency_ms":180,"jitter_ms":35,"packet_loss_pct":3,"dns_latency_ms":300}`)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Score    int `json:"network_score"`
		Findings []struct {
			Code string `json:"code"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	if result.Score >= 70 || len(result.Findings) == 0 {
		t.Fatalf("unexpected analysis: %s", raw)
	}
}

func TestAnalyzeNetworkJSONRejectsUnknownFields(t *testing.T) {
	if _, err := AnalyzeNetworkJSON(`{"network_type":"wifi","download_mbps":1,"upload_mbps":1,"latency_ms":1,"jitter_ms":0,"packet_loss_pct":0,"dns_latency_ms":1,"sim_serial":"secret"}`); err == nil {
		t.Fatal("unknown field should be rejected")
	}
}
