package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
