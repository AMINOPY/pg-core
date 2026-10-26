// Package profileimport converts supported share links and NPV JSON exports into
// Xray JSON without attempting to decode password-protected proprietary bundles.
package profileimport

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

const maxProfileBytes = 4 << 20

// ToXrayJSON accepts raw Xray JSON, NPV JSON wrappers containing embedded Xray
// JSON, and standard VLESS/VMess/Trojan/Shadowsocks share links. Unsupported
// transports are rejected instead of silently dropping security-sensitive fields.
func ToXrayJSON(input []byte) ([]byte, error) {
	if len(input) == 0 || len(input) > maxProfileBytes {
		return nil, errors.New("profile must be between 1 byte and 4 MiB")
	}
	s := strings.TrimSpace(string(input))
	if s == "" {
		return nil, errors.New("profile is empty")
	}
	if strings.HasPrefix(strings.ToLower(s), "slipnet://") {
		return nil, errors.New("SlipNet URI requires the SlipNet runtime; it is not an Xray profile")
	}
	if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
		cfg, err := extractJSON([]byte(s))
		if err != nil {
			return nil, err
		}
		return ensureLocalSOCKS(cfg)
	}
	for _, scheme := range []string{"vless://", "vmess://", "trojan://", "ss://"} {
		if strings.HasPrefix(strings.ToLower(s), scheme) {
			cfg, err := shareLinkToXray(s)
			if err != nil {
				return nil, err
			}
			return ensureLocalSOCKS(cfg)
		}
	}
	if strings.HasPrefix(strings.ToUpper(s), "NPVT1") || strings.HasSuffix(strings.ToLower(s), ".npvt") {
		return nil, errors.New("NPVT container detected; password-protected/proprietary NPV bundles are not decoded by PG CORE")
	}
	return nil, errors.New("unsupported profile format: expected Xray JSON, NPV JSON with embedded v2rayJson, or VLESS/VMess/Trojan/Shadowsocks URI")
}

func extractJSON(raw []byte) ([]byte, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, errors.New("profile JSON is invalid")
	}
	switch v := value.(type) {
	case map[string]any:
		if hasOutbounds(v) {
			return json.Marshal(v)
		}
		if nested, ok := v["v2rayProfile"].(map[string]any); ok {
			if rawCfg, ok := nested["v2rayJson"].(string); ok && strings.TrimSpace(rawCfg) != "" {
				return extractJSON([]byte(rawCfg))
			}
		}
		if nested, ok := v["v2rayJson"].(string); ok && strings.TrimSpace(nested) != "" {
			return extractJSON([]byte(nested))
		}
	case []any:
		if len(v) != 1 {
			return nil, errors.New("NPV JSON contains multiple profiles; select/export one profile at a time")
		}
		b, err := json.Marshal(v[0])
		if err != nil {
			return nil, errors.New("NPV profile JSON is invalid")
		}
		return extractJSON(b)
	}
	return nil, errors.New("JSON profile has no Xray outbounds or embedded v2rayJson; lossy conversion is refused")
}

func hasOutbounds(v map[string]any) bool {
	outbounds, ok := v["outbounds"].([]any)
	return ok && len(outbounds) > 0
}

func ensureLocalSOCKS(cfg []byte) ([]byte, error) {
	var root map[string]any
	if err := json.Unmarshal(cfg, &root); err != nil {
		return nil, errors.New("converted Xray profile is invalid")
	}
	outbounds, ok := root["outbounds"].([]any)
	if !ok || len(outbounds) == 0 {
		return nil, errors.New("Xray profile must contain at least one outbound")
	}
	if inbounds, ok := root["inbounds"].([]any); ok {
		for _, item := range inbounds {
			in, ok := item.(map[string]any)
			if !ok {
				return nil, errors.New("refusing to import malformed Xray inbound")
			}
			listen, _ := in["listen"].(string)
			listen = strings.TrimSpace(listen)
			ip := net.ParseIP(listen)
			if strings.EqualFold(listen, "localhost") {
				ip = net.ParseIP("127.0.0.1")
			}
			// An empty listen address commonly means a wildcard bind in Xray.
			// Reject every non-loopback inbound, not only SOCKS, so a profile
			// cannot accidentally expose an auxiliary listener to the network.
			if ip == nil || !ip.IsLoopback() {
				return nil, errors.New("refusing to import an Xray inbound that is not explicitly bound to loopback")
			}
			protocol, _ := in["protocol"].(string)
			if protocol != "socks" {
				continue
			}
			port, ok := in["port"].(float64)
			if !ok || port < 1 || port > 65535 || port != float64(int(port)) {
				return nil, errors.New("existing local SOCKS inbound has an invalid port")
			}
			settings, _ := in["settings"].(map[string]any)
			auth, _ := settings["auth"].(string)
			if auth != "" && auth != "noauth" {
				return nil, errors.New("existing local SOCKS inbound must use no-auth for this local-only runner")
			}
			return json.Marshal(root)
		}
	}
	root["inbounds"] = append(anySlice(root["inbounds"]), map[string]any{
		"tag": "pg-core-socks-in", "listen": "127.0.0.1", "port": 1080,
		"protocol": "socks", "settings": map[string]any{"auth": "noauth", "udp": false},
	})
	return json.Marshal(root)
}

func anySlice(v any) []any {
	if values, ok := v.([]any); ok {
		return values
	}
	return []any{}
}

func shareLinkToXray(raw string) ([]byte, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, errors.New("share URI is malformed")
	}
	protocol := strings.ToLower(u.Scheme)
	host := u.Hostname()
	port := 0
	if protocol != "vmess" {
		var err error
		port, err = strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 || host == "" || net.ParseIP(host) == nil && strings.ContainsAny(host, " /\\\\") {
			return nil, errors.New("share URI must include a valid server host and port")
		}
	}
	query := u.Query()
	name := u.Fragment
	if name == "" {
		name = "proxy"
	}
	stream := map[string]any{"network": "tcp"}
	outbound := map[string]any{"tag": "proxy", "streamSettings": stream}
	switch protocol {
	case "vless":
		id := ""
		if u.User != nil {
			id, _ = u.User.Password()
			if id == "" {
				id = u.User.Username()
			}
		}
		if id == "" {
			return nil, errors.New("VLESS URI is missing its UUID")
		}
		user := map[string]any{"id": id, "encryption": "none"}
		if flow := query.Get("flow"); flow != "" {
			user["flow"] = flow
		}
		outbound["protocol"] = "vless"
		outbound["settings"] = map[string]any{"vnext": []any{map[string]any{"address": host, "port": port, "users": []any{user}}}}
		if err := applyStream(query, stream); err != nil {
			return nil, err
		}
	case "trojan":
		password := ""
		if u.User != nil {
			password, _ = u.User.Password()
			if password == "" {
				password = u.User.Username()
			}
		}
		if password == "" {
			return nil, errors.New("Trojan URI is missing its password")
		}
		outbound["protocol"] = "trojan"
		outbound["settings"] = map[string]any{"servers": []any{map[string]any{"address": host, "port": port, "password": password}}}
		if err := applyStream(query, stream); err != nil {
			return nil, err
		}
	case "ss":
		if query.Get("plugin") != "" {
			return nil, errors.New("Shadowsocks plugin options are not supported by this importer")
		}
		if u.User == nil {
			return nil, errors.New("Shadowsocks URI is missing method and password")
		}
		method := u.User.Username()
		password, hasPassword := u.User.Password()
		if !hasPassword {
			decoded, decodeErr := decodeBase64(method)
			if decodeErr != nil {
				return nil, errors.New("Shadowsocks URI credentials are malformed")
			}
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) != 2 {
				return nil, errors.New("Shadowsocks URI must encode method:password")
			}
			method, password = parts[0], parts[1]
		}
		if method == "" || password == "" {
			return nil, errors.New("Shadowsocks URI is missing method or password")
		}
		outbound["protocol"] = "shadowsocks"
		outbound["settings"] = map[string]any{"servers": []any{map[string]any{"address": host, "port": port, "method": method, "password": password}}}
		stream["network"] = "tcp"
	case "vmess":
		payload := strings.SplitN(strings.TrimPrefix(raw, "vmess://"), "#", 2)[0]
		decoded, err := decodeBase64(payload)
		if err != nil {
			return nil, errors.New("VMess URI payload is not valid base64 JSON")
		}
		var vm struct {
			Address string          `json:"add"`
			Port    json.RawMessage `json:"port"`
			ID      string          `json:"id"`
			AlterID json.RawMessage `json:"aid"`
			Network string          `json:"net"`
			Host    string          `json:"host"`
			Path    string          `json:"path"`
			TLS     string          `json:"tls"`
			SNI     string          `json:"sni"`
		}
		if err := json.Unmarshal(decoded, &vm); err != nil || vm.Address == "" || vm.ID == "" {
			return nil, errors.New("VMess URI is missing address or UUID")
		}
		vmPort, err := rawInt(vm.Port)
		if err != nil || vmPort < 1 || vmPort > 65535 {
			return nil, errors.New("VMess URI has invalid port")
		}
		aid, _ := rawInt(vm.AlterID)
		outbound["protocol"] = "vmess"
		outbound["settings"] = map[string]any{"vnext": []any{map[string]any{"address": vm.Address, "port": vmPort, "users": []any{map[string]any{"id": vm.ID, "alterId": aid, "security": "auto"}}}}}
		q := url.Values{}
		q.Set("type", vm.Network)
		q.Set("host", vm.Host)
		q.Set("path", vm.Path)
		q.Set("security", vm.TLS)
		q.Set("sni", vm.SNI)
		if err := applyStream(q, stream); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported share URI scheme %q", protocol)
	}
	root := map[string]any{
		"log":       map[string]any{"loglevel": "warning"},
		"inbounds":  []any{map[string]any{"tag": "pg-core-socks-in", "listen": "127.0.0.1", "port": 1080, "protocol": "socks", "settings": map[string]any{"auth": "noauth", "udp": false}}},
		"outbounds": []any{outbound},
	}
	_ = name
	return json.Marshal(root)
}

func applyStream(query url.Values, stream map[string]any) error {
	network := query.Get("type")
	if network == "" {
		network = query.Get("network")
	}
	if network == "" {
		network = "tcp"
	}
	switch network {
	case "tcp", "ws", "grpc", "http", "h2":
	case "kcp", "quic":
		return fmt.Errorf("transport %q is not supported by this importer", network)
	default:
		return fmt.Errorf("unknown transport %q", network)
	}
	if network == "h2" {
		network = "http"
	}
	stream["network"] = network
	security := strings.ToLower(query.Get("security"))
	if security == "" && query.Get("tls") == "tls" {
		security = "tls"
	}
	if security == "tls" {
		tls := map[string]any{}
		if sni := query.Get("sni"); sni != "" {
			tls["serverName"] = sni
		} else if sni := query.Get("servername"); sni != "" {
			tls["serverName"] = sni
		}
		if fp := query.Get("fp"); fp != "" {
			tls["fingerprint"] = fp
		}
		if alpn := query.Get("alpn"); alpn != "" {
			tls["alpn"] = strings.Split(alpn, ",")
		}
		stream["security"] = "tls"
		stream["tlsSettings"] = tls
	} else if security == "reality" {
		reality := map[string]any{}
		if sni := query.Get("sni"); sni != "" {
			reality["serverName"] = sni
		}
		if key := query.Get("pbk"); key != "" {
			reality["publicKey"] = key
		}
		if sid := query.Get("sid"); sid != "" {
			reality["shortId"] = sid
		}
		if fp := query.Get("fp"); fp != "" {
			reality["fingerprint"] = fp
		}
		if reality["serverName"] == nil || reality["publicKey"] == nil {
			return errors.New("REALITY URI requires sni and pbk parameters")
		}
		stream["security"] = "reality"
		stream["realitySettings"] = reality
	} else if security != "" && security != "none" {
		return fmt.Errorf("unsupported security mode %q", security)
	}
	switch network {
	case "ws":
		ws := map[string]any{}
		if path := query.Get("path"); path != "" {
			ws["path"] = path
		}
		headers := map[string]any{}
		if host := query.Get("host"); host != "" {
			headers["Host"] = host
		}
		if len(headers) > 0 {
			ws["headers"] = headers
		}
		stream["wsSettings"] = ws
	case "grpc":
		grpc := map[string]any{}
		if service := query.Get("serviceName"); service != "" {
			grpc["serviceName"] = service
		} else if service := query.Get("service"); service != "" {
			grpc["serviceName"] = service
		}
		if authority := query.Get("authority"); authority != "" {
			grpc["authority"] = authority
		}
		stream["grpcSettings"] = grpc
	case "http":
		http := map[string]any{}
		if path := query.Get("path"); path != "" {
			http["path"] = path
		}
		if host := query.Get("host"); host != "" {
			http["host"] = strings.Split(host, ",")
		}
		stream["httpSettings"] = http
	}
	return nil
}

func decodeBase64(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{base64.RawURLEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.StdEncoding} {
		if b, err := enc.DecodeString(strings.TrimSpace(s)); err == nil {
			return b, nil
		}
	}
	return nil, errors.New("invalid base64")
}

func rawInt(raw json.RawMessage) (int, error) {
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strconv.Atoi(s)
	}
	return 0, errors.New("invalid integer")
}
