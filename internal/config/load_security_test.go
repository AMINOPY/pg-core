package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AMINOPY/pg-core/internal/security"
)

func TestLoadRejectsOversizedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := strings.Repeat(" ", int(MaxConfigBytes)+1)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "maximum size") {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func TestLoadRejectsUnknownFieldsAndMultipleValues(t *testing.T) {
	for name, data := range map[string]string{
		"unknown field":   `{"schema_version":1,"profile":"test","active_engine":"sing-box","extra":true}`,
		"multiple values": `{"schema_version":1,"profile":"test","active_engine":"sing-box"} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("expected untrusted config input to be rejected")
			}
		})
	}
}

func TestLoadVerifiedRejectsTampering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{"schema_version":1,"profile":"test","active_engine":"sing-box"}`)
	key := []byte("test-only-key")
	tag, err := security.Sign(key, data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadVerified(path, key, tag); err != nil {
		t.Fatalf("valid signed config rejected: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"profile":"changed","active_engine":"sing-box"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadVerified(path, key, tag); err == nil {
		t.Fatal("tampered config must be rejected")
	}
	if _, err := LoadVerified(path, nil, tag); err == nil {
		t.Fatal("empty key must be rejected")
	}
}
