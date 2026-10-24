package transportprofile

import (
	"crypto/ed25519"
	"testing"
	"time"
)

func FuzzVerifyManifest(f *testing.F) {
	f.Add([]byte(`{}`), "example.com", uint64(0))
	f.Add([]byte(`{"version":1}`), "example.com", uint64(1))
	f.Fuzz(func(t *testing.T, raw []byte, host string, minSequence uint64) {
		if len(raw) > MaxManifestBytes+1 {
			return
		}
		keys := map[string]ed25519.PublicKey{
			"fuzz-key": make(ed25519.PublicKey, ed25519.PublicKeySize),
		}
		_, _ = VerifyManifest(raw, host, keys, time.Unix(1_800_000_000, 0), minSequence)
	})
}
