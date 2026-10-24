package transportprofile

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func signedFixture(t *testing.T, now time.Time) (Manifest, ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	m := Manifest{
		Version:   ManifestVersion,
		KeyID:     "test-key-1",
		Host:      "tunnel.example.com",
		Sequence:  7,
		IssuedAt:  now.Add(-time.Minute),
		ExpiresAt: now.Add(time.Hour),
		Profiles: []Profile{{
			Version:    1,
			Name:       "primary",
			Endpoint:   "https://tunnel.example.com",
			ServerName: "tunnel.example.com",
			Kind:       WebSocket,
			Path:       "/transport",
		}},
	}
	m, err = SignManifest(m, priv)
	if err != nil {
		t.Fatal(err)
	}
	return m, pub, priv
}

func marshalManifest(t *testing.T, m Manifest) []byte {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestVerifySignedManifest(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	m, pub, _ := signedFixture(t, now)
	got, err := VerifyManifest(marshalManifest(t, m), "tunnel.example.com", map[string]ed25519.PublicKey{"test-key-1": pub}, now, 6)
	if err != nil {
		t.Fatalf("VerifyManifest() error = %v", err)
	}
	if got.Sequence != 7 || got.Profiles[0].Name != "primary" {
		t.Fatalf("unexpected manifest: %+v", got)
	}
}

func TestVerifyRejectsTamperingAndWrongHost(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	m, pub, _ := signedFixture(t, now)
	keys := map[string]ed25519.PublicKey{"test-key-1": pub}

	tampered := m
	tampered.Profiles = append([]Profile(nil), m.Profiles...)
	tampered.Profiles[0].Path = "/other"
	if _, err := VerifyManifest(marshalManifest(t, tampered), m.Host, keys, now, 0); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("tampered manifest error = %v, want ErrInvalidManifest", err)
	}
	if _, err := VerifyManifest(marshalManifest(t, m), "other.example.com", keys, now, 0); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("wrong host error = %v, want ErrInvalidManifest", err)
	}
}

func TestVerifyRejectsExpiredRollbackAndUnknownKey(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	m, pub, _ := signedFixture(t, now)
	keys := map[string]ed25519.PublicKey{"test-key-1": pub}
	raw := marshalManifest(t, m)
	if _, err := VerifyManifest(raw, m.Host, keys, now.Add(2*time.Hour), 0); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("expired manifest error = %v", err)
	}
	if _, err := VerifyManifest(raw, m.Host, keys, now, m.Sequence+1); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("rollback manifest error = %v", err)
	}
	if _, err := VerifyManifest(raw, m.Host, nil, now, 0); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("unknown key error = %v", err)
	}
}

func TestVerifyRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	m, pub, _ := signedFixture(t, now)
	raw := string(marshalManifest(t, m))
	withUnknown := strings.TrimSuffix(raw, "}") + `, "unexpected": true}`
	keys := map[string]ed25519.PublicKey{"test-key-1": pub}
	if _, err := VerifyManifest([]byte(withUnknown), m.Host, keys, now, 0); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("unknown field error = %v", err)
	}
	if _, err := VerifyManifest(append([]byte(raw), []byte(` {}`)...), m.Host, keys, now, 0); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("trailing JSON error = %v", err)
	}
}

func TestSignManifestRejectsInvalidPrivateKey(t *testing.T) {
	if _, err := SignManifest(Manifest{}, []byte("bad")); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("SignManifest() error = %v, want ErrInvalidManifest", err)
	}
}

func TestFetchManifestUsesWellKnownPathAndVerifiesTLSResponse(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	m, pub, priv := signedFixture(t, now)
	m.Host = "localhost"
	m, err := SignManifest(m, priv)
	if err != nil {
		t.Fatal(err)
	}
	body := marshalManifest(t, m)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != discoveryPath {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer server.Close()

	url := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	client := server.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	got, err := fetchManifestWithClient(context.Background(), url, map[string]ed25519.PublicKey{"test-key-1": pub}, now, 7, client)
	if err != nil {
		t.Fatalf("fetchManifestWithClient() error = %v", err)
	}
	if got.Host != "localhost" || got.Sequence != 7 {
		t.Fatalf("unexpected manifest: %+v", got)
	}
}

func TestFetchManifestRejectsRedirect(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://other.example.com/.well-known/pg-core/transport.json", http.StatusFound)
	}))
	defer server.Close()
	url := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	client := server.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	_, err := fetchManifestWithClient(context.Background(), url, nil, now, 0, client)
	if !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("fetchManifestWithClient() error = %v, want ErrInvalidManifest", err)
	}
}

func TestFetchManifestRejectsUnsafeURLs(t *testing.T) {
	now := time.Now()
	for _, url := range []string{
		"http://example.com",
		"https://user:pass@example.com",
		"https://example.com/path",
		"https://example.com:8443",
		"https://example.com/?x=1",
	} {
		t.Run(url, func(t *testing.T) {
			_, err := FetchManifest(context.Background(), url, nil, now, 0)
			if !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("FetchManifest() error = %v, want ErrInvalidManifest", err)
			}
		})
	}
}
