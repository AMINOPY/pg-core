package xray

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
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

func TestApplyFrontingRejectsTrailingJSON(t *testing.T) {
	a := New("xray")
	a.Fronting = &fronting.Settings{SNI: "cdn.example.com", EdgeIPs: []string{"104.16.0.1"}}
	cfg := []byte(`{"outbounds":[{"protocol":"vless","settings":{"vnext":[{"address":"origin.example.com"}]},"streamSettings":{"security":"tls"}}]} { "unexpected": true }`)
	if _, err := a.applyFronting(cfg); err == nil {
		t.Fatal("expected trailing JSON data to be rejected")
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
	for _, bad := range [][]byte{
		[]byte(`{"inbounds":[{"listen":"0.0.0.0","port":1080,"protocol":"socks","settings":{"auth":"noauth"}}]}`),
		[]byte(`{"inbounds":[{"listen":"127.0.0.1","port":1080,"protocol":"socks","settings":{"auth":"password","accounts":[{"user":"u","pass":"p"}]}}]}`),
	} {
		if _, err := proxyAddress(bad); err == nil {
			t.Fatalf("expected unsafe probe listener to be rejected: %s", bad)
		}
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

func TestHealthWithoutDataPathTargetIsDegraded(t *testing.T) {
	a := New("unused")
	a.proc = &processState{done: make(chan struct{})}
	report, err := a.Health(context.Background())
	if err != nil {
		t.Fatalf("Health returned error: %v", err)
	}
	if report.Status != engine.StatusDegraded || report.ErrorCode != "process_liveness_only" {
		t.Fatalf("process liveness must not imply connectivity: %+v", report)
	}
}

func TestHealthCanVerifyTCPDataPathThroughSOCKS(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		greeting := make([]byte, 3)
		if _, err := io.ReadFull(conn, greeting); err != nil {
			serverErr <- err
			return
		}
		if _, err := conn.Write([]byte{5, 0}); err != nil {
			serverErr <- err
			return
		}
		head := make([]byte, 5)
		if _, err := io.ReadFull(conn, head[:4]); err != nil {
			serverErr <- err
			return
		}
		if head[3] == 3 {
			if _, err := io.ReadFull(conn, head[4:5]); err != nil {
				serverErr <- err
				return
			}
			nameAndPort := make([]byte, int(head[4])+2)
			if _, err := io.ReadFull(conn, nameAndPort); err != nil {
				serverErr <- err
				return
			}
		} else {
			serverErr <- errors.New("expected domain-name SOCKS request")
			return
		}
		_, err = conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
		serverErr <- err
	}()

	a := New("unused")
	a.proc = &processState{
		done:         make(chan struct{}),
		proxyAddr:    listener.Addr().String(),
		healthTarget: "example.com:443",
	}
	report, err := a.Health(context.Background())
	if err != nil {
		t.Fatalf("Health returned error: %v", err)
	}
	if report.Status != engine.StatusHealthy || report.ErrorCode != "" {
		t.Fatalf("unexpected health report: %+v", report)
	}
	if err := <-serverErr; err != nil {
		t.Fatalf("SOCKS test server: %v", err)
	}
}

func TestHealthMarksFailedWhenDataPathProbeFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	a := New("unused")
	a.proc = &processState{done: make(chan struct{}), proxyAddr: addr, healthTarget: "example.com:443"}
	report, err := a.Health(context.Background())
	if err != nil {
		t.Fatalf("Health returned error: %v", err)
	}
	if report.Status != engine.StatusFailed || report.ErrorCode != "data_path_probe_failed" {
		t.Fatalf("unexpected health report: %+v", report)
	}
}

func TestValidateHealthTarget(t *testing.T) {
	for _, target := range []string{"example.com:443", "127.0.0.1:80", "[::1]:443"} {
		if err := validateHealthTarget(target); err != nil {
			t.Errorf("valid target %q rejected: %v", target, err)
		}
	}
	for _, target := range []string{"", "example.com", "example.com:0", "example.com:65536", "example.com:443\n"} {
		if err := validateHealthTarget(target); err == nil {
			t.Errorf("invalid target %q accepted", target)
		}
	}
}

func TestProbeViaSOCKSRejectsUnavailableProxy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := probeViaSOCKS(ctx, "127.0.0.1:1", "example.com:443"); err == nil {
		t.Fatal("expected probe failure")
	}
}

func TestLifecycleMethodsRejectNilContext(t *testing.T) {
	a := New("xray")
	cfg := []byte(`{"outbounds":[{"protocol":"freedom"}]}`)
	if err := a.ValidateConfig(nil, cfg); err == nil {
		t.Fatal("ValidateConfig should reject a nil context")
	}
	if err := a.Start(nil, cfg); err == nil {
		t.Fatal("Start should reject a nil context")
	}
	if _, err := a.Health(nil); err == nil {
		t.Fatal("Health should reject a nil context")
	}
	if err := a.Stop(nil); err == nil {
		t.Fatal("Stop should reject a nil context")
	}
}

func TestStartRejectsDuplicateProcessAndStopCleansUp(t *testing.T) {
	binary := fakeBinary(t, `if [ "$2" = "-test" ]; then exit 0; fi
trap 'exit 0' INT TERM
while :; do sleep 1; done`)
	a := New(binary)
	cfg := []byte(`{"outbounds":[{"protocol":"freedom"}]}`)
	if err := a.Start(context.Background(), cfg); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if err := a.Start(context.Background(), cfg); err != ErrAlreadyRunning {
		t.Fatalf("second Start error = %v, want ErrAlreadyRunning", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := a.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	report, err := a.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != engine.StatusStopped {
		t.Fatalf("status after Stop = %v, want stopped", report.Status)
	}
}

func TestApplyLocalSOCKSAddsLoopbackOnlyInbound(t *testing.T) {
	cfg := []byte(`{"outbounds":[{"protocol":"freedom"}]}`)
	got, err := applyLocalSOCKS(cfg, 18080)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(got, &root); err != nil {
		t.Fatal(err)
	}
	inbounds := root["inbounds"].([]any)
	if len(inbounds) != 1 {
		t.Fatalf("inbounds count = %d", len(inbounds))
	}
	in := inbounds[0].(map[string]any)
	if in["listen"] != "127.0.0.1" || in["protocol"] != "socks" || in["tag"] != "pgcore-local-socks" {
		t.Fatalf("unexpected local inbound: %#v", in)
	}
	settings := in["settings"].(map[string]any)
	if settings["auth"] != "noauth" || settings["udp"] != true {
		t.Fatalf("unexpected SOCKS settings: %#v", settings)
	}
}

func TestApplyLocalSOCKSDisabledAndPortValidation(t *testing.T) {
	cfg := []byte(`{"outbounds":[{"protocol":"freedom"}]}`)
	got, err := applyLocalSOCKS(cfg, 0)
	if err != nil || string(got) != string(cfg) {
		t.Fatalf("disabled setting should preserve profile: got=%s err=%v", got, err)
	}
	for _, port := range []int{-1, 65536} {
		if _, err := applyLocalSOCKS(cfg, port); err == nil {
			t.Errorf("expected invalid port %d to be rejected", port)
		}
	}
}

func TestApplyLocalSOCKSRejectsPortConflict(t *testing.T) {
	cfg := []byte(`{"inbounds":[{"listen":"0.0.0.0","port":18080,"protocol":"socks","settings":{"auth":"noauth"}}],"outbounds":[{"protocol":"freedom"}]}`)
	if _, err := applyLocalSOCKS(cfg, 18080); err == nil {
		t.Fatal("expected non-loopback listener conflict to be rejected")
	}
}

func TestPrepareConfigComposesFrontingAndLocalSOCKS(t *testing.T) {
	a := New("unused")
	a.Fronting = &fronting.Settings{SNI: "cdn.example.com", EdgeIPs: []string{"104.16.0.1"}}
	a.LocalSOCKSPort = 18080
	cfg := []byte(`{"outbounds":[{"protocol":"vless","settings":{"vnext":[{"address":"origin.example.com","port":443,"users":[{"id":"test-id"}]}]},"streamSettings":{"network":"ws","security":"tls","tlsSettings":{},"wsSettings":{"path":"/tunnel","headers":{"Host":"origin.example.com"}}}}]}`)
	got, err := a.prepareConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(got, &root); err != nil {
		t.Fatal(err)
	}
	inbounds := root["inbounds"].([]any)
	if len(inbounds) != 1 || inbounds[0].(map[string]any)["listen"] != "127.0.0.1" {
		t.Fatalf("local app inbound missing: %#v", inbounds)
	}
	out := root["outbounds"].([]any)[0].(map[string]any)
	server := out["settings"].(map[string]any)["vnext"].([]any)[0].(map[string]any)
	if server["address"] != "104.16.0.1" {
		t.Fatalf("fronting edge IP not applied: %#v", server)
	}
	tls := out["streamSettings"].(map[string]any)["tlsSettings"].(map[string]any)
	if tls["serverName"] != "cdn.example.com" {
		t.Fatalf("SNI not applied: %#v", tls)
	}
}

func TestLocalSOCKSAddressRequiresLoopbackNoAuth(t *testing.T) {
	good := []byte(`{"inbounds":[{"listen":"127.0.0.1","port":1080,"protocol":"socks","settings":{"auth":"noauth"}}]}`)
	addr, err := LocalSOCKSAddress(good)
	if err != nil || addr != "127.0.0.1:1080" {
		t.Fatalf("addr=%q err=%v", addr, err)
	}
	localhost := []byte(`{"inbounds":[{"listen":"localhost","port":1081,"protocol":"socks","settings":{"auth":"noauth"}}]}`)
	addr, err = LocalSOCKSAddress(localhost)
	if err != nil || addr != "127.0.0.1:1081" {
		t.Fatalf("localhost addr=%q err=%v", addr, err)
	}
	bad := []byte(`{"inbounds":[{"listen":"0.0.0.0","port":1080,"protocol":"socks","settings":{"auth":"noauth"}}]}`)
	if _, err := LocalSOCKSAddress(bad); err == nil {
		t.Fatal("expected public SOCKS listener to be rejected")
	}
}
