package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type EngineID string

const (
	EngineXray EngineID = "xray"
	EngineSingBox EngineID = "sing-box"
	EngineMihomo EngineID = "mihomo"
	EngineAether EngineID = "aether"
)

type Config struct {
	Profile string `json:"profile"`
	ActiveEngine EngineID `json:"active_engine"`
	Enabled []EngineID `json:"enabled_engines"`
	TUNOwner string `json:"tun_owner,omitempty"`
	DNSOwner string `json:"dns_owner,omitempty"`
}

func Load(path string) (Config, error) {
	var cfg Config
	f, err := os.Open(path)
	if err != nil { return cfg, fmt.Errorf("open config: %w", err) }
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil { return cfg, fmt.Errorf("parse JSON: %w", err) }
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Profile) == "" { return errors.New("profile must not be empty") }
	if !knownEngine(c.ActiveEngine) { return fmt.Errorf("active_engine %q is unsupported", c.ActiveEngine) }
	enabled := make(map[EngineID]bool, len(c.Enabled))
	for _, id := range c.Enabled {
		if !knownEngine(id) { return fmt.Errorf("enabled_engines contains unsupported engine %q", id) }
		if enabled[id] { return fmt.Errorf("enabled_engines contains duplicate %q", id) }
		enabled[id] = true
	}
	if len(enabled) > 0 && !enabled[c.ActiveEngine] {
		return errors.New("active_engine must be listed in enabled_engines when that list is provided")
	}
	if c.TUNOwner != "" && !knownEngine(EngineID(c.TUNOwner)) { return fmt.Errorf("tun_owner %q is unsupported", c.TUNOwner) }
	if c.DNSOwner != "" && !knownEngine(EngineID(c.DNSOwner)) { return fmt.Errorf("dns_owner %q is unsupported", c.DNSOwner) }
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
