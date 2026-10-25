package transportprofile

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var ErrSequenceRollback = errors.New("manifest sequence rollback rejected")
var ErrSequenceStore = errors.New("manifest sequence store error")

// SequenceStore persists the highest accepted manifest sequence per host.
// Implementations must reject Accept calls below the stored sequence.
// Equal sequences are allowed so clients can refresh the same signed manifest.
type SequenceStore interface {
	Current(host string) (uint64, error)
	Accept(host string, sequence uint64) error
}

// FileSequenceStore is a cross-platform rollback store. It serializes operations
// within this process and uses same-directory atomic replacement. Protect the
// parent directory from untrusted local users.
type FileSequenceStore struct {
	path string
	mu   sync.Mutex
}

type sequenceDocument struct {
	Version   int               `json:"version"`
	Sequences map[string]uint64 `json:"sequences"`
}

const sequenceStoreVersion = 1

func NewFileSequenceStore(path string) (*FileSequenceStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("%w: empty path", ErrSequenceStore)
	}
	return &FileSequenceStore{path: path}, nil
}

// Current returns the highest sequence persisted for host, or zero if absent.
func (s *FileSequenceStore) Current(host string) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	host, err := canonicalStoreHost(host)
	if err != nil {
		return 0, err
	}
	doc, err := s.readLocked()
	if err != nil {
		return 0, err
	}
	return doc.Sequences[host], nil
}

// Accept persists sequence unless it would roll back a previously accepted
// sequence. Call this only after the manifest signature and validity are verified.
func (s *FileSequenceStore) Accept(host string, sequence uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	host, err := canonicalStoreHost(host)
	if err != nil {
		return err
	}
	if sequence == 0 {
		return fmt.Errorf("%w: sequence must be positive", ErrSequenceStore)
	}
	doc, err := s.readLocked()
	if err != nil {
		return err
	}
	if previous := doc.Sequences[host]; sequence < previous {
		return fmt.Errorf("%w: host %s has %d, received %d", ErrSequenceRollback, host, previous, sequence)
	}
	if doc.Sequences[host] == sequence {
		return nil
	}
	if len(doc.Sequences) >= 256 {
		if _, exists := doc.Sequences[host]; !exists {
			return fmt.Errorf("%w: host capacity exceeded", ErrSequenceStore)
		}
	}
	doc.Sequences[host] = sequence
	return s.writeLocked(doc)
}

func canonicalStoreHost(host string) (string, error) {
	host = normalizeHost(host)
	if host == "" || !validDNSName(host) {
		return "", fmt.Errorf("%w: invalid host", ErrSequenceStore)
	}
	return host, nil
}

func (s *FileSequenceStore) readLocked() (sequenceDocument, error) {
	doc := sequenceDocument{Version: sequenceStoreVersion, Sequences: make(map[string]uint64)}
	info, err := os.Lstat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return doc, nil
	}
	if err != nil {
		return doc, fmt.Errorf("%w: inspect file: %v", ErrSequenceStore, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return doc, fmt.Errorf("%w: store must be a regular non-symlink file", ErrSequenceStore)
	}
	if info.Size() > 64<<10 {
		return doc, fmt.Errorf("%w: file exceeds 64 KiB", ErrSequenceStore)
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return doc, fmt.Errorf("%w: read file: %v", ErrSequenceStore, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return doc, fmt.Errorf("%w: decode file: %v", ErrSequenceStore, err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return doc, fmt.Errorf("%w: trailing JSON", ErrSequenceStore)
	}
	if doc.Version != sequenceStoreVersion || doc.Sequences == nil || len(doc.Sequences) > 256 {
		return doc, fmt.Errorf("%w: unsupported or invalid store document", ErrSequenceStore)
	}
	for host, seq := range doc.Sequences {
		if _, err := canonicalStoreHost(host); err != nil || seq == 0 {
			return doc, fmt.Errorf("%w: invalid stored entry", ErrSequenceStore)
		}
	}
	return doc, nil
}

func (s *FileSequenceStore) writeLocked(doc sequenceDocument) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("%w: create directory: %v", ErrSequenceStore, err)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("%w: encode file: %v", ErrSequenceStore, err)
	}
	tmp, err := os.CreateTemp(dir, ".pg-core-sequences-*")
	if err != nil {
		return fmt.Errorf("%w: create temporary file: %v", ErrSequenceStore, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("%w: secure temporary file: %v", ErrSequenceStore, err)
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("%w: write temporary file: %v", ErrSequenceStore, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("%w: sync temporary file: %v", ErrSequenceStore, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("%w: close temporary file: %v", ErrSequenceStore, err)
	}
	if err := replaceSequenceFile(tmpName, s.path); err != nil {
		return fmt.Errorf("%w: replace store: %v", ErrSequenceStore, err)
	}
	return nil
}

// FetchManifestWithStore verifies the manifest against the persisted sequence
// and only advances the sequence after signature, host, expiry, and profile
// validation succeed. Accept performs a second monotonicity check for concurrent
// fetches that use the same store implementation.
func FetchManifestWithStore(ctx context.Context, discoveryURL string, trustedKeys map[string]ed25519.PublicKey, now time.Time, store SequenceStore) (Manifest, error) {
	if store == nil {
		return Manifest{}, fmt.Errorf("%w: sequence store is required", ErrSequenceStore)
	}
	u, err := url.Parse(discoveryURL)
	if err != nil || u == nil || u.Hostname() == "" {
		return Manifest{}, fmt.Errorf("%w: invalid discovery URL", ErrInvalidManifest)
	}
	host := normalizeHost(u.Hostname())
	minSequence, err := store.Current(host)
	if err != nil {
		return Manifest{}, err
	}
	manifest, err := FetchManifest(ctx, discoveryURL, trustedKeys, now, minSequence)
	if err != nil {
		return Manifest{}, err
	}
	if err := store.Accept(host, manifest.Sequence); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}
