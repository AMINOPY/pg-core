package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactsSensitiveAttributes(t *testing.T) {
	var out bytes.Buffer
	logger := NewJSONLoggerWithWriter(&out, slog.LevelInfo)
	logger.Info("connection", "engine", "test", "access_token", "secret-value", "password", "p@ss", "latency_ms", 25)
	line := out.String()
	for _, secret := range []string{"secret-value", "p@ss"} {
		if strings.Contains(line, secret) {
			t.Fatalf("secret leaked in log: %s", line)
		}
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["access_token"] != "[REDACTED]" || got["password"] != "[REDACTED]" {
		t.Fatalf("redaction missing: %#v", got)
	}
	if got["engine"] != "test" {
		t.Fatalf("ordinary attribute lost: %#v", got)
	}
}

func TestRedactsNestedGroup(t *testing.T) {
	var out bytes.Buffer
	logger := NewJSONLoggerWithWriter(&out, slog.LevelInfo)
	logger.LogAttrs(context.Background(), slog.LevelInfo, "nested", slog.Group("auth", slog.String("private_key", "hidden")))
	if strings.Contains(out.String(), "hidden") || !strings.Contains(out.String(), "[REDACTED]") {
		t.Fatalf("nested redaction failed: %s", out.String())
	}
}
