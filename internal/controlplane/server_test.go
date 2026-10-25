package controlplane

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	keyA = "app-a-admin-key-0123456789abcdef0123456789"
	keyB = "app-b-read-key-0123456789abcdef0123456789"
)

func testServer(t *testing.T) (*Server, *MemoryStore) {
	t.Helper()
	store := NewMemoryStore()
	s, err := NewServer(store, []Credential{
		{Key: keyA, AppID: "app-a", Scopes: []string{ScopeRead, ScopeManageUsers, ScopeManageAnnouncements, ScopeManageReleases}},
		{Key: keyB, AppID: "app-b", Scopes: []string{ScopeRead}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, store
}

func request(t *testing.T, s *Server, method, path, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if key != "" {
		r.Header.Set("Authorization", "Bearer "+key)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestTenantIsolationForAnnouncements(t *testing.T) {
	s, _ := testServer(t)
	w := request(t, s, http.MethodPost, "/v1/admin/announcements", keyA, `{"message":"planned maintenance"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("publish status=%d body=%s", w.Code, w.Body.String())
	}
	w = request(t, s, http.MethodGet, "/v1/announcement", keyB, "")
	if w.Code != http.StatusOK {
		t.Fatalf("read other tenant status=%d", w.Code)
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if string(result["announcement"]) != "null" {
		t.Fatalf("cross-tenant data leaked: %s", w.Body.String())
	}
}

func TestReadOnlyKeyCannotWrite(t *testing.T) {
	s, _ := testServer(t)
	w := request(t, s, http.MethodPost, "/v1/admin/announcements", keyB, `{"message":"not allowed"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestUserStatusIsScopedAndValidated(t *testing.T) {
	s, _ := testServer(t)
	w := request(t, s, http.MethodPut, "/v1/admin/users/user-42", keyA, `{"status":"suspended"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("write status=%d body=%s", w.Code, w.Body.String())
	}
	w = request(t, s, http.MethodGet, "/v1/admin/users/user-42", keyB, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("other tenant key should lack scope, got %d", w.Code)
	}
	w = request(t, s, http.MethodPut, "/v1/admin/users/user-43", keyA, `{"status":"erase-device"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid status accepted: %d", w.Code)
	}
}

func TestStrictJSONAndReleaseURLValidation(t *testing.T) {
	s, _ := testServer(t)
	w := request(t, s, http.MethodPost, "/v1/admin/announcements", keyA, `{"message":"hello","unexpected":true}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown field accepted: %d", w.Code)
	}
	w = request(t, s, http.MethodPost, "/v1/admin/announcements", keyA, `{"message":"hello"} { "message":"second" }`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("trailing JSON accepted: %d", w.Code)
	}
	w = request(t, s, http.MethodPost, "/v1/admin/releases", keyA, `{"platform":"android","version":"1.0","url":"http://insecure.example/app.apk"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("insecure release URL accepted: %d", w.Code)
	}
}

func TestAnnouncementExpiry(t *testing.T) {
	s, store := testServer(t)
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	store.PutAnnouncement("app-a", Announcement{ID: "a1", Message: "expired", ExpiresAt: now.Add(-time.Second)})
	w := request(t, s, http.MethodGet, "/v1/announcement", keyA, "")
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"announcement":null`)) {
		t.Fatalf("expired announcement should be hidden: %d %s", w.Code, w.Body.String())
	}
}

func TestUnauthorizedAndInvalidCredentials(t *testing.T) {
	if _, err := NewServer(NewMemoryStore(), []Credential{{Key: "short", AppID: "app-a", Scopes: []string{ScopeRead}}}); err == nil {
		t.Fatal("short key accepted")
	}
	s, _ := testServer(t)
	w := request(t, s, http.MethodGet, "/v1/app", "wrong", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", w.Code)
	}
}
