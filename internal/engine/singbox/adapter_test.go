package singbox

import (
	"context"
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
	path := filepath.Join(t.TempDir(), "sing-box")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCapabilities(t *testing.T) {
	c := New("sing-box").Capabilities()
	if !c.TUN || !c.DNS || !c.Routing || !c.HealthChecks {
		t.Fatalf("capabilities: %+v", c)
	}
}

func TestValidateConfig(t *testing.T) {
	ok := New(fakeBinary(t, "[ \"$1\" = \"check\" ] && exit 0\nexit 1"))
	if err := ok.ValidateConfig(context.Background(), []byte("{}")); err != nil {
		t.Fatal(err)
	}
	bad := New(fakeBinary(t, "exit 2"))
	err := bad.ValidateConfig(context.Background(), []byte("{\"token\":\"secret-value\"}"))
	if err == nil || strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("unsafe validation error: %v", err)
	}
}

func TestLifecycle(t *testing.T) {
	binary := fakeBinary(t, "if [ \"$1\" = \"check\" ]; then exit 0; fi\nexec sleep 60")
	a := New(binary)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := a.Start(ctx, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if err := a.Start(ctx, []byte("{}")); err != ErrAlreadyRunning {
		t.Fatalf("second start: %v", err)
	}
	report, err := a.Health(ctx)
	if err != nil || report.Status != engine.StatusHealthy {
		t.Fatalf("health report=%+v err=%v", report, err)
	}
	if err := a.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	report, err = a.Health(ctx)
	if err != nil || report.Status != engine.StatusStopped {
		t.Fatalf("after stop report=%+v err=%v", report, err)
	}
}

func TestProxyAddressRequiresLoopbackUnauthenticatedSOCKS(t *testing.T) {
	good := []byte(`{"inbounds":[{"type":"socks","listen":"127.0.0.1","listen_port":1080} ]}`)
	addr, err := proxyAddress(good)
	if err != nil || addr != "127.0.0.1:1080" {
		t.Fatalf("addr=%q err=%v", addr, err)
	}
	for _, bad := range [][]byte{
		[]byte(`{"inbounds":[{"type":"socks","listen":"0.0.0.0","listen_port":1080}]}`),
		[]byte(`{"inbounds":[{"type":"socks","listen":"127.0.0.1","listen_port":1080,"users":[{"username":"u","password":"p"}]}]}`),
		[]byte(`{"inbounds":[{"type":"http","listen":"127.0.0.1","listen_port":1080}]}`),
	} {
		if _, err := proxyAddress(bad); err == nil {
			t.Fatalf("expected invalid probe inbound to be rejected: %s", bad)
		}
	}
}

func TestHealthMarksFailedWhenConfiguredDataPathProbeFails(t *testing.T) {
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

func TestRejectsInvalidInputBeforeExec(t *testing.T) {
	a := New("/does/not/exist")
	if err := a.ValidateConfig(context.Background(), nil); err == nil {
		t.Fatal("expected empty config error")
	}
	if err := a.ValidateConfig(context.Background(), make([]byte, maxConfigBytes+1)); err == nil {
		t.Fatal("expected oversized config error")
	}
}

func TestFrontingAppliedToStartedProfile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "seen.json")
	binary := fakeBinary(t, "if [ \"$1\" = \"check\" ]; then cp \"$3\" '"+out+"'; exit 0; fi\nexec sleep 60")
	a := New(binary)
	a.Fronting = &fronting.Settings{SNI: "cdn.example.com", EdgeIPs: []string{"104.16.0.1"}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	profile := `{"outbounds":[{"type":"trojan","tag":"p","server":"o.example.com","server_port":443,"password":"x","tls":{"enabled":true}}]}`
	if err := a.Start(ctx, []byte(profile)); err != nil {
		t.Fatal(err)
	}
	defer a.Stop(ctx)
	seen, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(seen), `"server":"104.16.0.1"`) || !strings.Contains(string(seen), `"server_name":"cdn.example.com"`) {
		t.Fatalf("profile not rewritten: %s", seen)
	}
}

func TestFrontingErrorWithoutTLSOutbound(t *testing.T) {
	a := New(fakeBinary(t, "exit 0"))
	a.Fronting = &fronting.Settings{SNI: "cdn.example.com", EdgeIPs: []string{"104.16.0.1"}}
	if err := a.ValidateConfig(context.Background(), []byte(`{"outbounds":[]}`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestAdapterRejectsNilContexts(t *testing.T) {
	a := New("/does/not/exist")
	if err := a.ValidateConfig(nil, []byte("{}")); err == nil {
		t.Fatal("ValidateConfig should reject nil context")
	}
	if err := a.Start(nil, []byte("{}")); err == nil {
		t.Fatal("Start should reject nil context")
	}
	if _, err := a.Health(nil); err == nil {
		t.Fatal("Health should reject nil context")
	}
	if err := a.Stop(nil); err == nil {
		t.Fatal("Stop should reject nil context")
	}
}
