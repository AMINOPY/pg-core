package config

import (
	"testing"

	"github.com/AMINOPY/pg-core/internal/fronting"
)

func TestValidateAcceptsKnownEngine(t *testing.T) {
	cfg := Config{
		Profile:      "development",
		ActiveEngine: EngineSingBox,
		Enabled:      []EngineID{EngineSingBox},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
}

func TestValidateRejectsUnknownEngine(t *testing.T) {
	cfg := Config{Profile: "test", ActiveEngine: EngineID("unknown")}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected unknown engine to be rejected")
	}
}

func TestValidateRequiresActiveEngineEnabled(t *testing.T) {
	cfg := Config{
		Profile:      "test",
		ActiveEngine: EngineXray,
		Enabled:      []EngineID{EngineMihomo},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected active engine mismatch to be rejected")
	}
}

func TestValidateRejectsDuplicateEngines(t *testing.T) {
	cfg := Config{
		Profile:      "test",
		ActiveEngine: EngineXray,
		Enabled:      []EngineID{EngineXray, EngineXray},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected duplicate engine to be rejected")
	}
}

func TestFrontingValidation(t *testing.T) {
	base := Config{SchemaVersion: 1, Profile: "p", ActiveEngine: EngineSingBox}
	good := base
	good.Fronting = &fronting.Settings{SNI: "cdn.example.com", EdgeIPs: []string{"104.16.0.1"}}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := base
	bad.Fronting = &fronting.Settings{SNI: "cdn.example.com"}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected error")
	}
}
