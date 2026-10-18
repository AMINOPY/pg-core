package config

import "testing"

func TestNormalizeLegacyConfigToV1(t *testing.T) {
	cfg, err := (Config{Profile:"dev", ActiveEngine:EngineSingBox}).Normalize()
	if err != nil { t.Fatal(err) }
	if cfg.SchemaVersion != CurrentSchemaVersion { t.Fatalf("version=%d", cfg.SchemaVersion) }
}
func TestRejectUnknownSchemaVersion(t *testing.T) {
	_, err := (Config{SchemaVersion:CurrentSchemaVersion+1}).Normalize()
	if err == nil { t.Fatal("expected unsupported version error") }
}
