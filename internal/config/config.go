package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const MaxConfigBytes int64 = 1 << 20 // 1 MiB hard limit for untrusted config input.

type EngineID string

const (
	CurrentSchemaVersion          = 1
	EngineXray           EngineID = "xray"
	EngineSingBox        EngineID = "sing-box"
	EngineMihomo         EngineID = "mihomo"
	EngineAether         EngineID = "aether"
)

type Config struct {
	SchemaVersion int        `json:"schema_version"`
	Profile       string     `json:"profile"`
	ActiveEngine  EngineID   `json:"active_engine"`
	Enabled       []EngineID `json:"enabled_engines"`
	TUNOwner      string     `json:"tun_owner,omitempty"`
	DNSOwner      string     `json:"dns_owner,omitempty"`
}

// Normalize applies safe, deterministic migrations for older schema versions.
// Version 0 represents pre-versioning configs and is upgraded in memory to v1.
func (c Config) Normalize() (Config, error) {
	if c.SchemaVersion == 0 {
		c.SchemaVersion = CurrentSchemaVersion
	}
	if c.SchemaVersion != CurrentSchemaVersion {
		return c, fmt.Errorf("unsupported schema_version %d (supported: %d)", c.SchemaVersion, CurrentSchemaVersion)
	}
	return c, nil
}

func Load(path string) (Config, error) {
	var cfg Config
	f, err := os.Open(path)
	if err != nil {
		return cfg, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	// Bound input before decoding so untrusted config files cannot force
	// unbounded memory use. Read one byte beyond the limit to detect overflow.
	data, err := io.ReadAll(io.LimitReader(f, MaxConfigBytes+1))
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if int64(len(data)) > MaxConfigBytes {
		return cfg, fmt.Errorf("config exceeds maximum size of %d bytes", MaxConfigBytes)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("parse JSON: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return cfg, errors.New("parse JSON: multiple JSON values are not allowed")
		}
		return cfg, fmt.Errorf("parse JSON trailer: %w", err)
	}
	cfg, err = cfg.Normalize()
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	normalized, err := c.Normalize()
	if err != nil {
		return err
	}
	c = normalized
	if strings.TrimSpace(c.Profile) == "" {
		return errors.New("profile must not be empty")
	}
	if !knownEngine(c.ActiveEngine) {
		return fmt.Errorf("active_engine %q is unsupported", c.ActiveEngine)
	}
	enabled := make(map[EngineID]bool, len(c.Enabled))
	for _, id := range c.Enabled {
		if !knownEngine(id) {
			return fmt.Errorf("enabled_engines contains unsupported engine %q", id)
		}
		if enabled[id] {
			return fmt.Errorf("enabled_engines contains duplicate %q", id)
		}
		enabled[id] = true
	}
	if len(enabled) > 0 && !enabled[c.ActiveEngine] {
		return errors.New("active_engine must be listed in enabled_engines when that list is provided")
	}
	if c.TUNOwner != "" && !knownEngine(EngineID(c.TUNOwner)) {
		return fmt.Errorf("tun_owner %q is unsupported", c.TUNOwner)
	}
	if c.DNSOwner != "" && !knownEngine(EngineID(c.DNSOwner)) {
		return fmt.Errorf("dns_owner %q is unsupported", c.DNSOwner)
	}
	return nil
}

func knownEngine(id EngineID) bool {
	switch id {
	case EngineXray, EngineSingBox, EngineMihomo, EngineAether:
		return true
	default:
		return false
	}
}
