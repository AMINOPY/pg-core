package transportprofile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileSequenceStoreMonotonicAndCanonicalHost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "sequences.json")
	store, err := NewFileSequenceStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := store.Current("TUNNEL.EXAMPLE.COM."); err != nil || got != 0 {
		t.Fatalf("Current initial = %d, %v", got, err)
	}
	if err := store.Accept("TUNNEL.EXAMPLE.COM.", 7); err != nil {
		t.Fatal(err)
	}
	if err := store.Accept("tunnel.example.com", 7); err != nil {
		t.Fatalf("equal sequence should be accepted: %v", err)
	}
	if err := store.Accept("tunnel.example.com", 6); !errors.Is(err, ErrSequenceRollback) {
		t.Fatalf("rollback error = %v, want ErrSequenceRollback", err)
	}
	reopened, err := NewFileSequenceStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := reopened.Current("tunnel.example.com"); err != nil || got != 7 {
		t.Fatalf("persisted Current = %d, %v", got, err)
	}
}

func TestFileSequenceStoreRejectsMalformedAndSymlinkFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sequences.json")
	if err := os.WriteFile(path, []byte(`{"version":2,"sequences":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, _ := NewFileSequenceStore(path)
	if _, err := store.Current("tunnel.example.com"); !errors.Is(err, ErrSequenceStore) {
		t.Fatalf("malformed store error = %v", err)
	}

	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte(`{"version":1,"sequences":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	symlinkStore, _ := NewFileSequenceStore(link)
	if _, err := symlinkStore.Current("tunnel.example.com"); !errors.Is(err, ErrSequenceStore) {
		t.Fatalf("symlink store error = %v", err)
	}
}

func TestFileSequenceStoreRejectsInvalidSequenceAndHost(t *testing.T) {
	store, _ := NewFileSequenceStore(filepath.Join(t.TempDir(), "sequences.json"))
	if err := store.Accept("tunnel.example.com", 0); !errors.Is(err, ErrSequenceStore) {
		t.Fatalf("zero sequence error = %v", err)
	}
	if err := store.Accept("not a host", 1); !errors.Is(err, ErrSequenceStore) {
		t.Fatalf("invalid host error = %v", err)
	}
}
