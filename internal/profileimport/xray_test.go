package profileimport

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestToXrayJSONAcceptsRawXrayAndAddsLoopbackSOCKS(t *testing.T) {
	got, err := ToXrayJSON([]byte(`{"outbounds":[{"protocol":"freedom","tag":"direct"}]}`))
	if err != nil { t.Fatal(err) }
	var cfg map[string]any
	if err := json.Unmarshal(got, &cfg); err != nil { t.Fatal(err) }
	inbounds := cfg["inbounds"].([]any)
	in := inbounds[0].(map[string]any)
	if in["listen"] != "127.0.0.1" || in["protocol"] != "socks" { t.Fatalf("unsafe inbound: %#v", in) }
}

func TestToXrayJSONExtractsNPVEmbeddedProfile(t *testing.T) {
	xrayJSON := `{"outbounds":[{"protocol":"freedom","tag":"direct"}]}`
	wrapper, _ := json.Marshal(map[string]any{"name":"fixture","type":"V2RAY","v2rayProfile":map[string]any{"v2rayJson":xrayJSON}})
	got, err := ToXrayJSON(wrapper)
	if err != nil { t.Fatal(err) }
	if !strings.Contains(string(got), "outbounds") || !strings.Contains(string(got), "pg-core-socks-in") { t.Fatalf("unexpected output: %s", got) }
}

func TestToXrayJSONConvertsVLESSWebSocketTLS(t *testing.T) {
	got, err := ToXrayJSON([]byte("vless://123e4567-e89b-12d3-a456-426614174000@example.com:443?type=ws&security=tls&sni=cdn.example&host=cdn.example&path=%2Fws#fixture"))
	if err != nil { t.Fatal(err) }
	var cfg map[string]any
	if err := json.Unmarshal(got, &cfg); err != nil { t.Fatal(err) }
	out := cfg["outbounds"].([]any)[0].(map[string]any)
	if out["protocol"] != "vless" { t.Fatalf("protocol = %v", out["protocol"]) }
	stream := out["streamSettings"].(map[string]any)
	if stream["network"] != "ws" || stream["security"] != "tls" { t.Fatalf("stream settings = %#v", stream) }
	tls := stream["tlsSettings"].(map[string]any)
	if tls["serverName"] != "cdn.example" { t.Fatalf("SNI = %v", tls["serverName"]) }
}

func TestToXrayJSONConvertsVMessBase64(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{"add":"example.com","port":"443","id":"123e4567-e89b-12d3-a456-426614174000","aid":"0","net":"tcp","tls":"tls","sni":"example.com"})
	link := "vmess://" + base64.RawStdEncoding.EncodeToString(payload)
	if _, err := ToXrayJSON([]byte(link)); err != nil { t.Fatal(err) }
}

func TestToXrayJSONRejectsUnsupportedOrLossyFormats(t *testing.T) {
	cases := []string{
		"slipnet://YWJj",
		"NPVT1\nsecret",
		`{"name":"locked","v2rayProfile":{"server":"example.com"}}`,
		`[{"v2rayProfile":{"v2rayJson":"{}"}},{"v2rayProfile":{"v2rayJson":"{}"}}]`,
	}
	for _, input := range cases {
		if _, err := ToXrayJSON([]byte(input)); err == nil { t.Errorf("expected rejection for %q", input) }
	}
}
