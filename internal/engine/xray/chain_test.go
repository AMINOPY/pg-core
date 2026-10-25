package xray

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestComposeHelperChainRoutesPrimaryThroughHelper(t *testing.T) {
	primary := []byte(`{"log":{"loglevel":"warning"},"inbounds":[{"tag":"local"}],"outbounds":[{"tag":"main","protocol":"vless","settings":{"vnext":[]}},{"tag":"direct","protocol":"freedom"}]}`)
	helper := []byte(`{"inbounds":[{"tag":"helper-local"}],"outbounds":[{"tag":"helper","protocol":"socks","settings":{"servers":[{"address":"127.0.0.1","port":1081}]}}],"dns":{"servers":["1.1.1.1"]}}`)
	got, err := ComposeHelperChain(primary, helper, "main", "helper")
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(got, &root); err != nil {
		t.Fatal(err)
	}
	if root["log"] == nil || root["dns"] != nil {
		t.Fatalf("primary config should be retained without importing helper-only settings: %s", got)
	}
	if len(root["inbounds"].([]any)) != 1 {
		t.Fatalf("helper inbound must not be imported: %s", got)
	}
	outbounds := root["outbounds"].([]any)
	if len(outbounds) != 3 {
		t.Fatalf("outbound count=%d, want 3", len(outbounds))
	}
	main := outbounds[0].(map[string]any)
	proxy := main["proxySettings"].(map[string]any)
	if proxy["tag"] != "helper" {
		t.Fatalf("proxySettings tag=%v, want helper", proxy["tag"])
	}
	if outbounds[2].(map[string]any)["tag"] != "helper" {
		t.Fatalf("helper outbound not appended: %s", got)
	}
}

func TestComposeHelperChainRejectsUnsafeOrAmbiguousProfiles(t *testing.T) {
	valid := []byte(`{"outbounds":[{"tag":"main","protocol":"freedom"}]}`)
	helper := []byte(`{"outbounds":[{"tag":"helper","protocol":"freedom"}]}`)
	cases := []struct {
		name               string
		primary, helper    []byte
		mainTag, helperTag string
	}{
		{"missing primary", valid, helper, "missing", "helper"},
		{"missing helper", valid, helper, "main", "missing"},
		{"tag collision", valid, []byte(`{"outbounds":[{"tag":"main","protocol":"freedom"}]}`), "main", "main"},
		{"trailing primary JSON", []byte(`{"outbounds":[{"tag":"main"}]} {}`), helper, "main", "helper"},
		{"trailing helper JSON", valid, []byte(`{"outbounds":[{"tag":"helper"}]} {}`), "main", "helper"},
		{"missing tags", []byte(`{"outbounds":[{"protocol":"freedom"}]}`), helper, "main", "helper"},
		{"nested helper", valid, []byte(`{"outbounds":[{"tag":"helper","proxySettings":{"tag":"next"}}]}`), "main", "helper"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ComposeHelperChain(tc.primary, tc.helper, tc.mainTag, tc.helperTag); err == nil {
				t.Fatal("expected profile to be rejected")
			}
		})
	}
}

func TestComposeHelperChainPreservesExistingPrimaryProxySettings(t *testing.T) {
	primary := []byte(`{"outbounds":[{"tag":"main","protocol":"vless","proxySettings":{"transportLayer":true}}]}`)
	helper := []byte(`{"outbounds":[{"tag":"helper","protocol":"freedom"}]}`)
	got, err := ComposeHelperChain(primary, helper, "main", "helper")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"transportLayer":true`) || !strings.Contains(string(got), `"tag":"helper"`) {
		t.Fatalf("expected existing proxy settings to be preserved and helper tag added: %s", got)
	}
}
