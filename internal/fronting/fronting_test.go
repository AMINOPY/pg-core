package fronting

import (
	"encoding/json"
	"strings"
	"testing"
)

const profile = `{
  "log": {"level": "warn"},
  "outbounds": [
    {"type": "vless", "tag": "proxy", "server": "origin.example.com", "server_port": 443,
     "uuid": "00000000-0000-0000-0000-000000000000",
     "tls": {"enabled": true, "server_name": "origin.example.com"},
     "transport": {"type": "ws", "path": "/ws"}},
    {"type": "direct", "tag": "direct"}
  ],
  "route": {"final": "proxy"}
}`

func outbounds(t *testing.T, b []byte) []map[string]any {
	t.Helper()
	var root struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(b, &root); err != nil {
		t.Fatal(err)
	}
	return root.Outbounds
}

func TestValidate(t *testing.T) {
	ok := []Settings{
		{},
		{SNI: "cdn.example.com", EdgeIPs: []string{"104.16.0.1", "2606:4700::1"}},
	}
	for _, s := range ok {
		if err := s.Validate(); err != nil {
			t.Errorf("%+v: %v", s, err)
		}
	}
	bad := []Settings{
		{SNI: "cdn.example.com"},
		{EdgeIPs: []string{"1.1.1.1"}},
		{SNI: "1.2.3.4", EdgeIPs: []string{"1.1.1.1"}},
		{SNI: "bad host", EdgeIPs: []string{"1.1.1.1"}},
		{SNI: "_service.cdn.example.com", EdgeIPs: []string{"1.1.1.1"}},
		{SNI: "cdn.example.com.", EdgeIPs: []string{"1.1.1.1"}},
		{SNI: "cdn.example.com", EdgeIPs: []string{"not-an-ip"}},
		{SNI: "cdn.example.com", EdgeIPs: []string{"23.649.251"}},
	}
	for _, s := range bad {
		if err := s.Validate(); err == nil {
			t.Errorf("%+v: expected error", s)
		}
	}
}

func TestApplyDisabledIsNoop(t *testing.T) {
	out, err := Apply([]byte(profile), Settings{})
	if err != nil || string(out) != profile {
		t.Fatalf("err=%v changed=%v", err, string(out) != profile)
	}
}

func TestApplySingleIP(t *testing.T) {
	out, err := Apply([]byte(profile), Settings{SNI: "cdn.example.com", EdgeIPs: []string{"104.16.0.1"}})
	if err != nil {
		t.Fatal(err)
	}
	obs := outbounds(t, out)
	if len(obs) != 2 {
		t.Fatalf("outbounds: %d", len(obs))
	}
	p := obs[0]
	if p["tag"] != "proxy" || p["server"] != "104.16.0.1" {
		t.Fatalf("proxy: %v", p)
	}
	if p["tls"].(map[string]any)["server_name"] != "cdn.example.com" {
		t.Fatalf("tls: %v", p["tls"])
	}
	if p["server_port"].(float64) != 443 || p["uuid"] == nil || p["transport"] == nil {
		t.Fatalf("other fields lost: %v", p)
	}
	if obs[1]["type"] != "direct" {
		t.Fatalf("direct outbound changed: %v", obs[1])
	}
}

func TestApplyMultipleIPs(t *testing.T) {
	out, err := Apply([]byte(profile), Settings{SNI: "cdn.example.com", EdgeIPs: []string{"104.16.0.1", "104.16.0.2"}})
	if err != nil {
		t.Fatal(err)
	}
	obs := outbounds(t, out)
	if len(obs) != 4 {
		t.Fatalf("outbounds: %d (%s)", len(obs), out)
	}
	if obs[0]["tag"] != "proxy-edge-1" || obs[0]["server"] != "104.16.0.1" ||
		obs[1]["tag"] != "proxy-edge-2" || obs[1]["server"] != "104.16.0.2" {
		t.Fatalf("members: %v %v", obs[0], obs[1])
	}
	if obs[2]["type"] != "urltest" || obs[2]["tag"] != "proxy" {
		t.Fatalf("urltest: %v", obs[2])
	}
	if !strings.Contains(string(out), `"final":"proxy"`) {
		t.Fatalf("route lost: %s", out)
	}
}

func TestApplyNeedsTLSOutbound(t *testing.T) {
	_, err := Apply([]byte(`{"outbounds":[{"type":"direct","tag":"direct"}]}`),
		Settings{SNI: "cdn.example.com", EdgeIPs: []string{"104.16.0.1"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestApplyRejectsBadJSON(t *testing.T) {
	if _, err := Apply([]byte(`[1]`), Settings{SNI: "cdn.example.com", EdgeIPs: []string{"1.1.1.1"}}); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseIPsSeparators(t *testing.T) {
	valid, skipped := ParseIPs([]string{"1.1.1.1, 8.8.8.8\n9.9.9.9;1.1.1.1", "23.649.251", "2606:4700::1"})
	want := []string{"1.1.1.1", "8.8.8.8", "9.9.9.9", "2606:4700::1"}
	if len(valid) != len(want) {
		t.Fatalf("valid: %v", valid)
	}
	for i := range want {
		if valid[i] != want[i] {
			t.Fatalf("valid: %v", valid)
		}
	}
	if len(skipped) != 1 || skipped[0] != "23.649.251" {
		t.Fatalf("skipped: %v", skipped)
	}
}

func TestBadEntriesAreSkippedNotFatal(t *testing.T) {
	s := Settings{SNI: "cdn.example.com", EdgeIPs: []string{"1.1.1.1", "oops", "1.1.1.1"}}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	out, err := Apply([]byte(profile), s)
	if err != nil {
		t.Fatal(err)
	}
	if obs := outbounds(t, out); obs[0]["server"] != "1.1.1.1" || len(obs) != 2 {
		t.Fatalf("unexpected: %s", out)
	}
}

func TestJSONStringOrArray(t *testing.T) {
	var a, b Settings
	if err := json.Unmarshal([]byte(`{"sni":"c.example.com","edge_ips":"1.1.1.1, 8.8.8.8\n9.9.9.9"}`), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"sni":"c.example.com","edge_ips":["1.1.1.1","8.8.8.8"]}`), &b); err != nil {
		t.Fatal(err)
	}
	if v, _ := ParseIPs(a.EdgeIPs); len(v) != 3 {
		t.Fatalf("a: %v", v)
	}
	if v, _ := ParseIPs(b.EdgeIPs); len(v) != 2 {
		t.Fatalf("b: %v", v)
	}
}
