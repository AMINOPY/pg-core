// Package fronting adds the CDN/SNI capability to the core: two inputs only,
// an SNI host name and a list of edge IP addresses.
//
// When both are set, every TLS outbound of a sing-box profile is pointed at the
// listed IPs (no DNS lookup of the server name) and announces the configured
// SNI in the TLS ClientHello. Certificate verification is left untouched, so
// the edge must present a valid certificate for that SNI. Whether the CDN
// forwards the traffic to the real server depends on its own routing rules and
// on the rest of the profile (for example the WebSocket Host header and path).
package fronting

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

const maxEdgeIPs = 64

// IPList is the edge IP list. In JSON it may be an array of strings or one
// string whose entries are separated by comma, semicolon, space or new line.
type IPList []string

func (l *IPList) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*l = IPList{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return errors.New("edge_ips must be a string or an array of strings")
	}
	*l = IPList(many)
	return nil
}

// Settings is the whole feature: SNI + IP list.
type Settings struct {
	SNI     string `json:"sni"`
	EdgeIPs IPList `json:"edge_ips"`
}

// ParseIPs splits raw entries on comma, semicolon, space or new line and
// returns the usable IPs (deduplicated, in order) and the entries that were
// skipped because they are not IP addresses. Any valid IP is accepted: there
// is no check against a list of CDN ranges.
func ParseIPs(entries []string) (valid, skipped []string) {
	seen := map[netip.Addr]bool{}
	for _, e := range entries {
		for _, part := range strings.FieldsFunc(e, func(r rune) bool {
			return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\r' || r == '\t'
		}) {
			addr, err := netip.ParseAddr(part)
			if err != nil {
				skipped = append(skipped, part)
				continue
			}
			addr = addr.Unmap()
			if seen[addr] {
				continue
			}
			seen[addr] = true
			valid = append(valid, addr.String())
		}
	}
	return valid, skipped
}

// Enabled reports whether an SNI and at least one usable IP are present.
func (s Settings) Enabled() bool {
	ips, _ := ParseIPs(s.EdgeIPs)
	return strings.TrimSpace(s.SNI) != "" && len(ips) > 0
}

// Validate checks the inputs. An entirely empty Settings is valid (feature off).
// Entries that are not IPs and duplicates are skipped, not rejected; an error
// is returned only when no usable IP remains.
func (s Settings) Validate() error {
	sni := strings.TrimSpace(s.SNI)
	ips, _ := ParseIPs(s.EdgeIPs)
	if sni == "" && len(s.EdgeIPs) == 0 {
		return nil
	}
	if sni == "" {
		return errors.New("fronting.sni is required when edge_ips is set")
	}
	if err := validHostname(sni); err != nil {
		return fmt.Errorf("fronting.sni: %w", err)
	}
	if len(ips) == 0 {
		return errors.New("fronting.edge_ips has no valid IP address")
	}
	for _, rawIP := range ips {
		addr, err := netip.ParseAddr(rawIP)
		if err != nil {
			return errors.New("fronting.edge_ips contains an invalid IP address")
		}
		addr = addr.Unmap()
		if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() ||
			addr.IsLinkLocalUnicast() || addr.IsMulticast() || addr.IsUnspecified() {
			return fmt.Errorf("fronting.edge_ips must contain public unicast addresses; %q is not public", rawIP)
		}
	}
	if len(ips) > maxEdgeIPs {
		return fmt.Errorf("fronting.edge_ips has more than %d usable entries", maxEdgeIPs)
	}
	return nil
}

func validHostname(h string) error {
	if len(h) > 253 {
		return errors.New("host name too long")
	}
	if _, err := netip.ParseAddr(h); err == nil {
		return errors.New("must be a host name, not an IP address")
	}
	for _, label := range strings.Split(strings.TrimSuffix(h, "."), ".") {
		if label == "" || len(label) > 63 {
			return errors.New("invalid host name")
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return errors.New("invalid host name")
		}
		for _, r := range label {
			ok := r == '-' || r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
			if !ok {
				return errors.New("invalid host name")
			}
		}
	}
	return nil
}

// Apply rewrites a complete sing-box JSON profile.
//
// For each outbound that has a TLS section:
//   - one IP: "server" becomes that IP and tls.server_name becomes the SNI;
//   - several IPs: one outbound per IP is generated ("<tag>-edge-N") and a
//     urltest outbound keeps the original tag, so routes and the rest of the
//     profile keep working and the fastest reachable edge IP is used.
//
// If Settings is not enabled the profile is returned unchanged. If it is
// enabled but the profile has no TLS outbound, an error is returned instead of
// silently doing nothing.
func Apply(profile []byte, s Settings) ([]byte, error) {
	if !s.Enabled() {
		return profile, nil
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	sni := strings.TrimSpace(s.SNI)
	ips, _ := ParseIPs(s.EdgeIPs)

	dec := json.NewDecoder(bytes.NewReader(profile))
	dec.UseNumber()
	var root map[string]any
	if err := dec.Decode(&root); err != nil {
		return nil, errors.New("profile is not a valid JSON object")
	}
	outbounds, _ := root["outbounds"].([]any)

	var out []any
	changed := 0
	for _, item := range outbounds {
		ob, ok := item.(map[string]any)
		if !ok || !hasTLS(ob) {
			out = append(out, item)
			continue
		}
		changed++
		if len(ips) == 1 {
			out = append(out, rewrite(ob, ips[0], sni, ""))
			continue
		}
		tag, _ := ob["tag"].(string)
		if tag == "" {
			return nil, errors.New("TLS outbound needs a tag when several edge IPs are used")
		}
		members := make([]any, 0, len(ips))
		for i, ip := range ips {
			memberTag := fmt.Sprintf("%s-edge-%d", tag, i+1)
			out = append(out, rewrite(ob, ip, sni, memberTag))
			members = append(members, memberTag)
		}
		out = append(out, map[string]any{
			"type":      "urltest",
			"tag":       tag,
			"outbounds": members,
			"url":       "https://www.gstatic.com/generate_204",
			"interval":  "3m",
		})
	}
	if changed == 0 {
		return nil, errors.New("fronting is set but the profile has no TLS outbound to apply it to")
	}
	root["outbounds"] = out
	return json.Marshal(root)
}

func hasTLS(ob map[string]any) bool {
	switch ob["type"] {
	case "direct", "block", "dns", "selector", "urltest":
		return false
	}
	tls, ok := ob["tls"].(map[string]any)
	if !ok {
		return false
	}
	if enabled, ok := tls["enabled"].(bool); ok && !enabled {
		return false
	}
	_, hasServer := ob["server"].(string)
	return hasServer
}

func rewrite(ob map[string]any, ip, sni, newTag string) map[string]any {
	cp := make(map[string]any, len(ob))
	for k, v := range ob {
		cp[k] = v
	}
	tls := make(map[string]any)
	for k, v := range ob["tls"].(map[string]any) {
		tls[k] = v
	}
	tls["enabled"] = true
	tls["server_name"] = sni
	cp["tls"] = tls
	cp["server"] = ip
	if newTag != "" {
		cp["tag"] = newTag
	}
	return cp
}
