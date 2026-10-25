package controlplane

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStorePersistsTenantScopedRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "controlplane.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	announcement := Announcement{ID: "notice-1", Message: "update soon", CreatedAt: now}
	if err := store.PutAnnouncement("app-a", announcement); err != nil {
		t.Fatal(err)
	}
	if err := store.PutRelease("app-a", Release{Platform: "android", Version: "1.2.0", URL: "https://example.com/app.apk", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.PutUserStatus("app-a", UserStatus{UserID: "u-1", Status: "suspended", UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}

	reopened, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	gotAnnouncement, ok, err := reopened.GetAnnouncement("app-a", now)
	if err != nil || !ok || gotAnnouncement.Message != announcement.Message {
		t.Fatalf("announcement persistence failed: %#v %v %v", gotAnnouncement, ok, err)
	}
	gotRelease, ok, err := reopened.GetRelease("app-a", "android")
	if err != nil || !ok || gotRelease.Version != "1.2.0" {
		t.Fatalf("release persistence failed: %#v %v %v", gotRelease, ok, err)
	}
	gotUser, ok, err := reopened.GetUserStatus("app-a", "u-1")
	if err != nil || !ok || gotUser.Status != "suspended" {
		t.Fatalf("user persistence failed: %#v %v %v", gotUser, ok, err)
	}
	if _, ok, err := reopened.GetAnnouncement("app-b", now); err != nil || ok {
		t.Fatalf("tenant data crossed: ok=%v err=%v", ok, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("state file permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestFileStoreRejectsCorruptState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{"unexpected":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileStore(path); err == nil {
		t.Fatal("corrupt state accepted")
	}
}
