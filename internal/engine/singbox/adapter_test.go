package singbox

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
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

func TestHealthReportsUnexpectedProcessExit(t *testing.T) {
	a := New(fakeBinary(t, "if [ \"$1\" = \"check\" ]; then exit 0; fi\nexit 7"))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := a.Start(ctx, []byte("{}")); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		report, err := a.Health(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if report.Status == engine.StatusFailed {
			if report.ErrorCode != "process_exited" {
				t.Fatalf("unexpected error code: %+v", report)
			}
			if err := a.Stop(ctx); err != nil {
				t.Fatal(err)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("health did not report the exited process")
}

func TestHealthHonorsCancelledContext(t *testing.T) {
	a := New("sing-box")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.Health(ctx); err == nil {
		t.Fatal("expected cancelled context error")
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
