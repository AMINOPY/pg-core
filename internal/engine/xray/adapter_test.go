package xray

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AMINOPY/pg-core/internal/fronting"
)

func fakeBinary(t *testing.T, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX script test")
	}
	path := filepath.Join(t.TempDir(), "xray")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCapabilitiesAndID(t *testing.T) {
	a := New("xray")
	if a.ID() != "xray" || a.Capabilities().TUN {
		t.Fatalf("unexpected adapter metadata: id=%q caps=%+v", a.ID(), a.Capabilities())
	}
}

func TestApplyFrontingPreservesCredentialsAndSetsSNI(t *testing.T) {
	a := New("xray")
	a.Fronting = &fronting.Settings{SNI: "cdn.example.com", EdgeIPs: []string{"104.16.0.1"}}
	raw := []byte(`{"outbounds":[{"protocol":"vless","settings":{"vnext":[{"address":"origin.example.com","port":443,"users":[{"id":"test-id"}]}]},"streamSettings":{"network":"ws","security":"tls","tlsSettings":{"serverName":"old.example.com"},"wsSettings":{"path":"/tunnel","headers":{"Host":"origin.example.com"}}}}]}`)
	out, err := a.applyFronting(raw)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	ob := got["outbounds"].([]any)[0].(map[string]any)
	settings := ob["settings"].(map[string]any)
	server := settings["vnext"].([]any)[0].(map[string]any)
	if server["address"] != "104.16.0.1" {
		t.Fatalf("address = %v", server["address"])
	}
	stream := ob["streamSettings"].(map[string]any)
	tlsSettings := stream["tlsSettings"].(map[string]any)
	if tlsSettings["serverName"] != "cdn.example.com" {
		t.Fatalf("serverName = %v", tlsSettings["serverName"])
	}
	users := server["users"].([]any)
	if users[0].(map[string]any)["id"] != "test-id" {
		t.Fatal("user credentials were not preserved")
	}
	ws := stream["wsSettings"].(map[string]any)
	if ws["path"] != "/tunnel" || ws["headers"].(map[string]any)["Host"] != "origin.example.com" {
		t.Fatal("transport settings were not preserved")
	}
}

func TestApplyFrontingRequiresTLSOutboundAndSingleIP(t *testing.T) {
	a := New("xray")
	a.Fronting = &fronting.Settings{SNI: "cdn.example.com", EdgeIPs: []string{"104.16.0.1"}}
	if _, err := a.applyFronting([]byte(`{"outbounds":[{"protocol":"freedom"}]}`)); err == nil {
		t.Fatal("expected missing TLS outbound error")
	}
	a.Fronting.EdgeIPs = []string{"104.16.0.1", "104.16.0.2"}
	if _, err := a.applyFronting([]byte(`{"outbounds":[]}`)); err == nil {
		t.Fatal("expected multi-IP-per-attempt to be rejected")
	}
}

func TestProxyAddressRequiresLoopbackSOCKS(t *testing.T) {
	good := []byte(`{"inbounds":[{"listen":"127.0.0.1","port":1080,"protocol":"socks","settings":{"auth":"noauth"}}]}`)
	addr, err := proxyAddress(good)
	if err != nil || addr != "127.0.0.1:1080" {
		t.Fatalf("addr=%q err=%v", addr, err)
	}
	bad := []byte(`{"inbounds":[{"listen":"0.0.0.0","port":1080,"protocol":"socks"}]}`)
	if _, err := proxyAddress(bad); err == nil {
		t.Fatal("expected non-loopback listener to be rejected")
	}
}

func TestValidateConfigUsesInstalledBinaryAndDoesNotLeakConfig(t *testing.T) {
	a := New(fakeBinary(t, "if [ \"$1\" = \"run\" ] && [ \"$2\" = \"-test\" ]; then exit 0; fi\nexit 2"))
	cfg := []byte(`{"inbounds":[{"listen":"127.0.0.1","port":1080,"protocol":"socks","settings":{"auth":"noauth"}}],"outbounds":[{"protocol":"freedom"}]}`)
	if err := a.ValidateConfig(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	bad := New(fakeBinary(t, "exit 2"))
	err := bad.ValidateConfig(context.Background(), []byte(`{"secret":"dont-leak-this"}`))
	if err == nil || strings.Contains(err.Error(), "dont-leak-this") {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestProbeViaSOCKSRejectsUnavailableProxy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := probeViaSOCKS(ctx, "127.0.0.1:1", "example.com:443"); err == nil {
		t.Fatal("expected probe failure")
	}
}
