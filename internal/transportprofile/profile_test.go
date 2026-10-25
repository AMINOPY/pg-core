package transportprofile

import (
	"errors"
	"testing"
)

func validProfile() Profile {
	return Profile{
		Version:    1,
		Name:       "primary",
		Endpoint:   "https://tunnel.example.com",
		ServerName: "tunnel.example.com",
		Kind:       WebSocket,
		Path:       "/transport",
		AuthRef:    "env:PGCORE_TOKEN",
	}
}

func TestValidProfile(t *testing.T) {
	if err := validProfile().Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestRejectsServerNameMismatch(t *testing.T) {
	p := validProfile()
	p.ServerName = "cdn.example.net"
	if err := p.Validate(); !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("Validate() error = %v, want ErrInvalidProfile", err)
	}
}

func TestRejectsInsecureOrAmbiguousEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
	}{
		{"plain HTTP", "http://tunnel.example.com"},
		{"user info", "https://user:pass@tunnel.example.com"},
		{"query", "https://tunnel.example.com?next=example.org"},
		{"fragment", "https://tunnel.example.com#fragment"},
		{"nonstandard port", "https://tunnel.example.com:8443"},
		{"IP literal", "https://192.0.2.1"},
		{"path must be separate", "https://tunnel.example.com/transport"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validProfile()
			p.Endpoint = tt.endpoint
			if err := p.Validate(); !errors.Is(err, ErrInvalidProfile) {
				t.Fatalf("Validate() error = %v, want ErrInvalidProfile", err)
			}
		})
	}
}

func TestRejectsInvalidKindPathAndAuthReference(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Profile)
	}{
		{"unknown kind", func(p *Profile) { p.Kind = "stealth-magic" }},
		{"relative path", func(p *Profile) { p.Path = "transport" }},
		{"authority-like path", func(p *Profile) { p.Path = "//attacker.example/path" }},
		{"query in path", func(p *Profile) { p.Path = "/transport?next=example.org" }},
		{"fragment in path", func(p *Profile) { p.Path = "/transport#fragment" }},
		{"backslash in path", func(p *Profile) { p.Path = "/transport\\admin" }},
		{"line break in path", func(p *Profile) { p.Path = "/transport\r\nHost: attacker.example" }},
		{"raw secret not allowed", func(p *Profile) { p.AuthRef = "super-secret-token" }},
		{"invalid environment reference", func(p *Profile) { p.AuthRef = "env:bad-name" }},
		{"invalid version", func(p *Profile) { p.Version = 2 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validProfile()
			tt.edit(&p)
			if err := p.Validate(); !errors.Is(err, ErrInvalidProfile) {
				t.Fatalf("Validate() error = %v, want ErrInvalidProfile", err)
			}
		})
	}
}

func TestEndpointHostComparisonIsCaseInsensitive(t *testing.T) {
	p := validProfile()
	p.Endpoint = "https://Tunnel.Example.Com"
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
