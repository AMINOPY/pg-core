package config

import (
	"testing"

	"github.com/AMINOPY/pg-core/internal/transportprofile"
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

func TestValidateConnectionModes(t *testing.T) {
	cfg := Config{
		Profile:      "development",
		ActiveEngine: EngineSingBox,
		Connection: &transportprofile.ConnectionConfig{
			Mode:       transportprofile.ModeCDNFronting,
			Endpoint:   "https://origin.example.com",
			ServerName: "edge.example.net",
			CDNIPs:     []string{"192.0.2.10"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid CDN-fronting config, got %v", err)
	}
	cfg.Connection.CDNIPs = nil
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected CDN-fronting config without IPs to be rejected")
	}
}
