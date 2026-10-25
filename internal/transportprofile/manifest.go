package transportprofile

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	ManifestVersion  = 1
	MaxManifestBytes = 256 << 10
	MaxProfiles      = 32
	MaxManifestTTL   = 24 * time.Hour
	maxClockSkew     = 5 * time.Minute
	discoveryPath    = "/.well-known/pg-core/transport.json"
)

var ErrInvalidManifest = errors.New("invalid transport manifest")

// Manifest is a signed, short-lived list of transport profiles. Host binds the
// manifest to the operator-controlled discovery hostname. Sequence supports
// caller-managed rollback protection.
type Manifest struct {
	Version   int       `json:"version"`
	KeyID     string    `json:"key_id"`
	Host      string    `json:"host"`
	Sequence  uint64    `json:"sequence"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Profiles  []Profile `json:"profiles"`
	Signature string    `json:"signature"`
}

type unsignedManifest struct {
	Version   int       `json:"version"`
	KeyID     string    `json:"key_id"`
	Host      string    `json:"host"`
	Sequence  uint64    `json:"sequence"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Profiles  []Profile `json:"profiles"`
}

func (m Manifest) signingBytes() ([]byte, error) {
	unsigned := unsignedManifest{
		Version: m.Version, KeyID: m.KeyID, Host: m.Host,
		Sequence: m.Sequence, IssuedAt: m.IssuedAt.UTC(),
		ExpiresAt: m.ExpiresAt.UTC(), Profiles: m.Profiles,
	}
	return json.Marshal(unsigned)
}

// SignManifest signs a manifest with Ed25519. The private key must be kept by
// the service operator and never shipped in the client.
func SignManifest(m Manifest, privateKey ed25519.PrivateKey) (Manifest, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return Manifest{}, fmt.Errorf("%w: invalid signing key", ErrInvalidManifest)
	}
	m.Signature = ""
	payload, err := m.signingBytes()
	if err != nil {
		return Manifest{}, fmt.Errorf("%w: encode: %v", ErrInvalidManifest, err)
	}
	m.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
	return m, nil
}

// VerifyManifest strictly parses and verifies a signed manifest. expectedHost
// must be the host from the HTTPS discovery URL. minSequence is the last
// accepted sequence number persisted by the caller; pass zero on first use.
func VerifyManifest(raw []byte, expectedHost string, trustedKeys map[string]ed25519.PublicKey, now time.Time, minSequence uint64) (Manifest, error) {
	var m Manifest
	if len(raw) == 0 || len(raw) > MaxManifestBytes {
		return m, fmt.Errorf("%w: document size must be 1..%d bytes", ErrInvalidManifest, MaxManifestBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return m, fmt.Errorf("%w: JSON: %v", ErrInvalidManifest, err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return m, fmt.Errorf("%w: trailing JSON value", ErrInvalidManifest)
	}
	if m.Version != ManifestVersion || m.Sequence == 0 || m.Sequence < minSequence {
		return m, fmt.Errorf("%w: unsupported version or rollback sequence", ErrInvalidManifest)
	}
	host := normalizeHost(expectedHost)
	if host == "" || normalizeHost(m.Host) != host || !validDNSName(host) {
		return m, fmt.Errorf("%w: host binding mismatch", ErrInvalidManifest)
	}
	if m.KeyID == "" || len(m.KeyID) > 64 {
		return m, fmt.Errorf("%w: invalid key id", ErrInvalidManifest)
	}
	key := trustedKeys[m.KeyID]
	if len(key) != ed25519.PublicKeySize {
		return m, fmt.Errorf("%w: untrusted signing key", ErrInvalidManifest)
	}
	now = now.UTC()
	issued := m.IssuedAt.UTC()
	expires := m.ExpiresAt.UTC()
	if issued.IsZero() || expires.IsZero() || issued.After(now.Add(maxClockSkew)) ||
		!expires.After(now) || !expires.After(issued) || expires.Sub(issued) > MaxManifestTTL {
		return m, fmt.Errorf("%w: invalid or expired validity window", ErrInvalidManifest)
	}
	if len(m.Profiles) == 0 || len(m.Profiles) > MaxProfiles {
		return m, fmt.Errorf("%w: profile count must be 1..%d", ErrInvalidManifest, MaxProfiles)
	}
	seen := make(map[string]struct{}, len(m.Profiles))
	for _, p := range m.Profiles {
		if err := p.Validate(); err != nil {
			return m, fmt.Errorf("%w: profile: %v", ErrInvalidManifest, err)
		}
		name := strings.ToLower(strings.TrimSpace(p.Name))
		if _, ok := seen[name]; ok {
			return m, fmt.Errorf("%w: duplicate profile name", ErrInvalidManifest)
		}
		seen[name] = struct{}{}
	}
	sig, err := base64.RawURLEncoding.DecodeString(m.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return m, fmt.Errorf("%w: invalid signature encoding", ErrInvalidManifest)
	}
	payload, err := m.signingBytes()
	if err != nil || !ed25519.Verify(key, payload, sig) {
		return m, fmt.Errorf("%w: signature verification failed", ErrInvalidManifest)
	}
	return m, nil
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	if strings.Contains(host, ":") {
		if u, err := url.Parse("https://" + host); err == nil {
			host = u.Hostname()
		}
	}
	return strings.TrimSuffix(host, ".")
}

// FetchManifest downloads the standard well-known document using verified HTTPS.
// Redirects are rejected to prevent discovery from silently changing authority.
func FetchManifest(ctx context.Context, discoveryURL string, trustedKeys map[string]ed25519.PublicKey, now time.Time, minSequence uint64) (Manifest, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return fetchManifestWithClient(ctx, discoveryURL, trustedKeys, now, minSequence, client)
}

func fetchManifestWithClient(ctx context.Context, discoveryURL string, trustedKeys map[string]ed25519.PublicKey, now time.Time, minSequence uint64, client *http.Client) (Manifest, error) {
	var zero Manifest
	u, err := url.Parse(discoveryURL)
	if err != nil || u == nil || u.Scheme != "https" || u.Hostname() == "" ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		(u.Path != "" && u.Path != discoveryPath) {
		return zero, fmt.Errorf("%w: discovery URL must be HTTPS and use the well-known path", ErrInvalidManifest)
	}
	if port := u.Port(); port != "" && port != "443" {
		return zero, fmt.Errorf("%w: discovery URL must use port 443", ErrInvalidManifest)
	}
	host := normalizeHost(u.Hostname())
	if !validDNSName(host) {
		return zero, fmt.Errorf("%w: invalid discovery hostname", ErrInvalidManifest)
	}
	u.Path = discoveryPath
	u.RawPath = ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return zero, fmt.Errorf("%w: request: %v", ErrInvalidManifest, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Chimera-Core/manifest-v1")
	resp, err := client.Do(req)
	if err != nil {
		return zero, fmt.Errorf("%w: fetch failed: %v", ErrInvalidManifest, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("%w: discovery returned HTTP %d", ErrInvalidManifest, resp.StatusCode)
	}
	if resp.ContentLength > MaxManifestBytes {
		return zero, fmt.Errorf("%w: response too large", ErrInvalidManifest)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxManifestBytes+1))
	if err != nil {
		return zero, fmt.Errorf("%w: read response: %v", ErrInvalidManifest, err)
	}
	if len(body) > MaxManifestBytes {
		return zero, fmt.Errorf("%w: response too large", ErrInvalidManifest)
	}
	return VerifyManifest(body, host, trustedKeys, now, minSequence)
}
