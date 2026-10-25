// Package transportprofile validates explicit, customer-controlled HTTPS transport
// profiles. It does not perform domain fronting or bypass CDN provider policy.
package transportprofile

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

type Kind string

const (
	HTTPS     Kind = "https"
	WebSocket Kind = "websocket"
	GRPC      Kind = "grpc"
)

var ErrInvalidProfile = errors.New("invalid transport profile")

var envReference = regexp.MustCompile(`^env:[A-Z_][A-Z0-9_]{0,63}$`)

// Profile describes a TLS endpoint operated by, or explicitly authorized for,
// the user. ServerName must match the HTTPS URL hostname so TLS identity
// verification remains end-to-end and does not rely on third-party impersonation.
type Profile struct {
	Version    int    `json:"version"`
	Name       string `json:"name"`
	Endpoint   string `json:"endpoint"`
	ServerName string `json:"server_name"`
	Kind       Kind   `json:"kind"`
	Path       string `json:"path"`
	AuthRef    string `json:"auth_ref,omitempty"`
}

func (p Profile) Validate() error {
	if p.Version != 1 {
		return fmt.Errorf("%w: version must be 1", ErrInvalidProfile)
	}
	if strings.TrimSpace(p.Name) == "" || len(p.Name) > 80 {
		return fmt.Errorf("%w: name must contain 1..80 non-space characters", ErrInvalidProfile)
	}
	if p.Kind != HTTPS && p.Kind != WebSocket && p.Kind != GRPC {
		return fmt.Errorf("%w: unsupported transport kind %q", ErrInvalidProfile, p.Kind)
	}
	u, err := url.Parse(p.Endpoint)
	if err != nil {
		return fmt.Errorf("%w: endpoint URL: %v", ErrInvalidProfile, err)
	}
	if u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("%w: endpoint must be an https origin without user info, query, or fragment", ErrInvalidProfile)
	}
	if u.Path != "" && u.Path != "/" {
		return fmt.Errorf("%w: endpoint must contain only the origin; use path for the transport path", ErrInvalidProfile)
	}
	if port := u.Port(); port != "" && port != "443" {
		return fmt.Errorf("%w: endpoint must use HTTPS port 443", ErrInvalidProfile)
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	serverName := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(p.ServerName)), ".")
	if !validDNSName(host) || !validDNSName(serverName) {
		return fmt.Errorf("%w: endpoint host and server_name must be valid DNS names", ErrInvalidProfile)
	}
	if host != serverName {
		return fmt.Errorf("%w: server_name must match endpoint hostname", ErrInvalidProfile)
	}
	if p.Path == "" || !strings.HasPrefix(p.Path, "/") || strings.HasPrefix(p.Path, "//") ||
		len(p.Path) > 1024 || strings.ContainsAny(p.Path, "\r\n\\?#") {
		return fmt.Errorf("%w: path must be an absolute path of at most 1024 bytes", ErrInvalidProfile)
	}
	if p.AuthRef != "" && !envReference.MatchString(p.AuthRef) {
		return fmt.Errorf("%w: auth_ref must reference an environment variable (env:NAME), never contain a secret", ErrInvalidProfile)
	}
	return nil
}

func validDNSName(name string) bool {
	if name == "" || len(name) > 253 || net.ParseIP(name) != nil {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				return false
			}
		}
	}
	return true
}
