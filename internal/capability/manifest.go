// Package capability defines the versioned, untrusted manifest format used
// to describe connector capabilities. A valid manifest is not proof that a
// connector is safe; policy and runtime checks are still required.
package capability

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const (
	SchemaVersion = "1.0"
	MaxManifestBytes = 256 << 10
	MaxCapabilities = 128
	MaxSchemaBytes = 64 << 10
)

var (
	idPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,127}$`)
	permissionPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9_-]*){0,5}$`)
	ErrManifestTooLarge = errors.New("capability manifest exceeds size limit")
)

// Manifest is supplied by a connector and must be treated as untrusted input.
// It deliberately contains no executable code or credentials.
type Manifest struct {
	SchemaVersion string       `json:"schema_version"`
	ConnectorID   string       `json:"connector_id"`
	ConnectorVersion string    `json:"connector_version"`
	Capabilities  []Capability `json:"capabilities"`
}

type Capability struct {
	ID            string          `json:"id"`
	Version       string          `json:"version"`
	InputSchema   json.RawMessage `json:"input_schema"`
	OutputSchema  json.RawMessage `json:"output_schema"`
	Permissions   []string        `json:"permissions"`
	Risk          string          `json:"risk"`
	Execution     string          `json:"execution"`
}

// ParseManifest bounds input, rejects unknown fields and trailing JSON values,
// and validates the manifest's structural invariants. It does not authorize
// permissions, execute code, or prove that a connector behaves as declared.
func ParseManifest(data []byte) (Manifest, error) {
	if len(data) > MaxManifestBytes {
		return Manifest{}, ErrManifestTooLarge
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("decode capability manifest: %w", err)
	}
	var trailing any
	if !errors.Is(decodeTrailing(dec, &trailing), io.EOF) {
		return Manifest{}, errors.New("multiple or invalid trailing JSON values")
	}
	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func decodeTrailing(dec *json.Decoder, v any) error { return dec.Decode(v) }

func (m Manifest) Validate() error {
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schema_version %q", m.SchemaVersion)
	}
	if !idPattern.MatchString(m.ConnectorID) {
		return errors.New("connector_id has invalid format")
	}
	if !validVersion(m.ConnectorVersion) {
		return errors.New("connector_version must be a non-empty bounded identifier")
	}
	if len(m.Capabilities) == 0 || len(m.Capabilities) > MaxCapabilities {
		return fmt.Errorf("capabilities count must be between 1 and %d", MaxCapabilities)
	}
	seen := make(map[string]struct{}, len(m.Capabilities))
	for i, c := range m.Capabilities {
		if !idPattern.MatchString(c.ID) {
			return fmt.Errorf("capabilities[%d].id has invalid format", i)
		}
		if _, ok := seen[c.ID]; ok {
			return fmt.Errorf("duplicate capability id %q", c.ID)
		}
		seen[c.ID] = struct{}{}
		if !validVersion(c.Version) {
			return fmt.Errorf("capabilities[%d].version is invalid", i)
		}
		if err := validateSchema("input_schema", c.InputSchema); err != nil {
			return fmt.Errorf("capabilities[%d]: %w", i, err)
		}
		if err := validateSchema("output_schema", c.OutputSchema); err != nil {
			return fmt.Errorf("capabilities[%d]: %w", i, err)
		}
		if len(c.Permissions) > 32 {
			return fmt.Errorf("capabilities[%d] has too many permissions", i)
		}
		permSeen := make(map[string]struct{}, len(c.Permissions))
		for _, p := range c.Permissions {
			if !permissionPattern.MatchString(p) {
				return fmt.Errorf("capabilities[%d] has invalid permission identifier", i)
			}
			if _, ok := permSeen[p]; ok {
				return fmt.Errorf("capabilities[%d] has duplicate permission %q", i, p)
			}
			permSeen[p] = struct{}{}
		}
		switch c.Risk {
		case "low", "medium", "high", "critical":
		default:
			return fmt.Errorf("capabilities[%d].risk is invalid", i)
		}
		switch c.Execution {
		case "in_process", "sandboxed", "remote":
		default:
			return fmt.Errorf("capabilities[%d].execution is invalid", i)
		}
		if c.Execution == "in_process" && c.Risk != "low" {
			return fmt.Errorf("capabilities[%d]: in_process execution is allowed only for low-risk declarations", i)
		}
	}
	return nil
}

func validateSchema(name string, raw json.RawMessage) error {
	if len(raw) == 0 || len(raw) > MaxSchemaBytes {
		return fmt.Errorf("%s is missing or too large", name)
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return fmt.Errorf("%s must be a JSON object", name)
	}
	typ, ok := value["type"]
	if !ok {
		return fmt.Errorf("%s must declare type", name)
	}
	var schemaType string
	if err := json.Unmarshal(typ, &schemaType); err != nil || schemaType != "object" {
		return fmt.Errorf("%s type must be object", name)
	}
	return nil
}

func validVersion(v string) bool {
	return v != "" && len(v) <= 64 && strings.TrimSpace(v) == v && !strings.ContainsAny(v, "\r\n\x00")
}
