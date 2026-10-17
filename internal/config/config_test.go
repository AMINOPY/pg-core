package config

import "testing"

func TestValidateAcceptsKnownEngine(t *testing.T) {
	cfg := Config{Profile:"development", ActiveEngine:EngineSingBox, Enabled:[]EngineID{EngineSingBox}}
	if err := cfg.Validate(); err != nil { t.Fatalf("expected valid config, got %v", err) }
}
func TestValidateRejectsUnknownEngine(t *testing.T) {
	cfg := Config{Profile:"test", ActiveEngine:EngineID("unknown")}
	if err := cfg.Validate(); err == nil { t.Fatal("expected unknown engine to be rejected") }
}
func TestValidateRequiresActiveEngineEnabled(t *testing.T) {
	cfg := Config{Profile:"test", ActiveEngine:EngineXray, Enabled:[]EngineID{EngineMihomo}}
	if err := cfg.Validate(); err == nil { t.Fatal("expected active engine mismatch to be rejected") }
}
func TestValidateRejectsDuplicateEngines(t *testing.T) {
	cfg := Config{Profile:"test", ActiveEngine:EngineXray, Enabled:[]EngineID{EngineXray,EngineXray}}
	if err := cfg.Validate(); err == nil { t.Fatal("expected duplicate engine to be rejected") }
}
