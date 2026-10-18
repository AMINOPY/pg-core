package capability

import (
	"strings"
	"testing"
)

const validManifest = `{
  "schema_version":"1.0",
  "connector_id":"example-assistant",
  "connector_version":"1.2.0",
  "capabilities":[{
    "id":"text.summarize",
    "version":"1.0",
    "input_schema":{"type":"object","properties":{"text":{"type":"string"}}},
    "output_schema":{"type":"object","properties":{"summary":{"type":"string"}}},
    "permissions":["model.inference"],
    "risk":"low",
    "execution":"sandboxed"
  }]
}`

func TestParseManifestValid(t *testing.T) {
	m, err := ParseManifest([]byte(validManifest))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if m.ConnectorID != "example-assistant" || len(m.Capabilities) != 1 {
		t.Fatalf("unexpected manifest: %#v", m)
	}
}

func TestParseManifestRejectsUnknownField(t *testing.T) {
	input := strings.Replace(validManifest, `"schema_version":"1.0",`, `"schema_version":"1.0","trusted":true,`, 1)
	if _, err := ParseManifest([]byte(input)); err == nil {
		t.Fatal("expected unknown field to be rejected")
	}
}

func TestParseManifestRejectsTrailingValue(t *testing.T) {
	if _, err := ParseManifest([]byte(validManifest + ` {}`)); err == nil {
		t.Fatal("expected trailing JSON value to be rejected")
	}
}

func TestManifestRejectsDuplicateCapability(t *testing.T) {
	m, err := ParseManifest([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}
	m.Capabilities = append(m.Capabilities, m.Capabilities[0])
	if err := m.Validate(); err == nil {
		t.Fatal("expected duplicate capability to be rejected")
	}
}

func TestParseManifestRejectsOversize(t *testing.T) {
	input := make([]byte, MaxManifestBytes+1)
	if _, err := ParseManifest(input); err != ErrManifestTooLarge {
		t.Fatalf("expected ErrManifestTooLarge, got %v", err)
	}
}

func TestManifestRejectsHighRiskInProcessExecution(t *testing.T) {
	m, err := ParseManifest([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}
	m.Capabilities[0].Risk = "high"
	m.Capabilities[0].Execution = "in_process"
	if err := m.Validate(); err == nil {
		t.Fatal("expected high-risk in-process declaration to be rejected")
	}
}

func FuzzParseManifestNeverPanics(f *testing.F) {
	f.Add([]byte(validManifest))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"schema_version":"1.0","capabilities":null}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > MaxManifestBytes+1 {
			return
		}
		_, _ = ParseManifest(data)
	})
}
