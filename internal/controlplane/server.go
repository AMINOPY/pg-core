// Package controlplane provides a tenant-scoped remote-management HTTP API.
// It deliberately manages app metadata and account flags, not arbitrary device data.
package controlplane

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxBodyBytes = 1 << 20

var (
	appIDPattern  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)
	itemIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:@-]{0,127}$`)
	ErrInvalid    = errors.New("invalid control-plane input")
)

// Scope names are capabilities. Keys should receive only the scopes they need.
const (
	ScopeRead                = "public:read"
	ScopeManageUsers         = "users:write"
	ScopeManageAnnouncements = "announcements:write"
	ScopeManageReleases      = "releases:write"
)

// Credential binds one secret to exactly one application tenant.
type Credential struct {
	Key    string // At least 32 random bytes encoded as a string; never log it.
	AppID  string
	Scopes []string
}

type Announcement struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

type Release struct {
	Platform  string    `json:"platform"`
	Version   string    `json:"version"`
	URL       string    `json:"url"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Force     bool      `json:"force"`
}

type UserStatus struct {
	UserID    string    `json:"user_id"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store interface {
	GetAnnouncement(appID string, now time.Time) (Announcement, bool)
	PutAnnouncement(appID string, value Announcement)
	GetRelease(appID, platform string) (Release, bool)
	PutRelease(appID string, value Release)
	GetUserStatus(appID, userID string) (UserStatus, bool)
	PutUserStatus(appID string, value UserStatus)
}

// MemoryStore is useful for tests and single-process demos. It is not durable:
// use a persistent Store implementation before production deployment.
type MemoryStore struct {
	mu            sync.RWMutex
	announcements map[string]Announcement
	releases      map[string]map[string]Release
	users         map[string]map[string]UserStatus
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		announcements: make(map[string]Announcement),
		releases:      make(map[string]map[string]Release),
		users:         make(map[string]map[string]UserStatus),
	}
}

func (s *MemoryStore) GetAnnouncement(appID string, now time.Time) (Announcement, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.announcements[appID]
	if ok && !v.ExpiresAt.IsZero() && !now.Before(v.ExpiresAt) {
		return Announcement{}, false
	}
	return v, ok
}

func (s *MemoryStore) PutAnnouncement(appID string, v Announcement) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.announcements[appID] = v
}

func (s *MemoryStore) GetRelease(appID, platform string) (Release, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.releases[appID][platform]
	return v, ok
}

func (s *MemoryStore) PutRelease(appID string, v Release) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.releases[appID] == nil {
		s.releases[appID] = make(map[string]Release)
	}
	s.releases[appID][v.Platform] = v
}

func (s *MemoryStore) GetUserStatus(appID, userID string) (UserStatus, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.users[appID][userID]
	return v, ok
}

func (s *MemoryStore) PutUserStatus(appID string, v UserStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.users[appID] == nil {
		s.users[appID] = make(map[string]UserStatus)
	}
	s.users[appID][v.UserID] = v
}

type credentialRecord struct {
	appID  string
	scopes map[string]bool
	hash   [32]byte
}

type Server struct {
	store Store
	keys  map[string]credentialRecord
	now   func() time.Time
}

func NewServer(store Store, credentials []Credential) (*Server, error) {
	if store == nil {
		return nil, errors.New("controlplane: store is required")
	}
	s := &Server{store: store, keys: make(map[string]credentialRecord), now: time.Now}
	for _, c := range credentials {
		if len(c.Key) < 32 || !appIDPattern.MatchString(c.AppID) {
			return nil, errors.New("controlplane: each key must be at least 32 characters and have a valid app ID")
		}
		h := sha256.Sum256([]byte(c.Key))
		id := hex.EncodeToString(h[:])
		if _, exists := s.keys[id]; exists {
			return nil, errors.New("controlplane: duplicate API key")
		}
		scopes := make(map[string]bool, len(c.Scopes))
		for _, scope := range c.Scopes {
			scopes[scope] = true
		}
		s.keys[id] = credentialRecord{appID: c.AppID, scopes: scopes, hash: h}
	}
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	cred, ok := s.authenticate(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/app":
		if !s.require(w, cred, ScopeRead) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"app_id": cred.appID, "api_version": "v1"})
	case r.Method == http.MethodGet && r.URL.Path == "/v1/announcement":
		if !s.require(w, cred, ScopeRead) {
			return
		}
		v, found := s.store.GetAnnouncement(cred.appID, s.now())
		if !found {
			writeJSON(w, http.StatusOK, map[string]any{"announcement": nil})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"announcement": v})
	case r.Method == http.MethodGet && r.URL.Path == "/v1/releases/latest":
		if !s.require(w, cred, ScopeRead) {
			return
		}
		platform := r.URL.Query().Get("platform")
		if !validPlatform(platform) {
			writeError(w, http.StatusBadRequest, "invalid_platform")
			return
		}
		v, found := s.store.GetRelease(cred.appID, platform)
		if !found {
			writeJSON(w, http.StatusOK, map[string]any{"release": nil})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"release": v})
	case r.Method == http.MethodPost && r.URL.Path == "/v1/admin/announcements":
		if !s.require(w, cred, ScopeManageAnnouncements) {
			return
		}
		var req struct {
			Message   string     `json:"message"`
			ExpiresAt *time.Time `json:"expires_at,omitempty"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		req.Message = strings.TrimSpace(req.Message)
		if req.Message == "" || len(req.Message) > 4000 {
			writeError(w, http.StatusBadRequest, "invalid_message")
			return
		}
		if req.ExpiresAt != nil && !req.ExpiresAt.After(s.now()) {
			writeError(w, http.StatusBadRequest, "expiry_must_be_future")
			return
		}
		v := Announcement{ID: randomID(), Message: req.Message, CreatedAt: s.now()}
		if req.ExpiresAt != nil {
			v.ExpiresAt = *req.ExpiresAt
		}
		s.store.PutAnnouncement(cred.appID, v)
		writeJSON(w, http.StatusCreated, map[string]any{"announcement": v})
	case r.Method == http.MethodPost && r.URL.Path == "/v1/admin/releases":
		if !s.require(w, cred, ScopeManageReleases) {
			return
		}
		var req struct {
			Platform string `json:"platform"`
			Version  string `json:"version"`
			URL      string `json:"url"`
			Notes    string `json:"notes"`
			Force    bool   `json:"force"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		req.Platform = strings.ToLower(strings.TrimSpace(req.Platform))
		req.Version = strings.TrimSpace(req.Version)
		if !validPlatform(req.Platform) || req.Version == "" || len(req.Version) > 64 || len(req.Notes) > 4000 || !validReleaseURL(req.URL) {
			writeError(w, http.StatusBadRequest, "invalid_release")
			return
		}
		v := Release{Platform: req.Platform, Version: req.Version, URL: req.URL, Notes: req.Notes, Force: req.Force, CreatedAt: s.now()}
		s.store.PutRelease(cred.appID, v)
		writeJSON(w, http.StatusCreated, map[string]any{"release": v})
	case strings.HasPrefix(r.URL.Path, "/v1/admin/users/"):
		if r.Method != http.MethodGet && r.Method != http.MethodPut {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		if !s.require(w, cred, ScopeManageUsers) {
			return
		}
		userID := strings.TrimPrefix(r.URL.Path, "/v1/admin/users/")
		if !itemIDPattern.MatchString(userID) || strings.Contains(userID, "/") {
			writeError(w, http.StatusBadRequest, "invalid_user_id")
			return
		}
		if r.Method == http.MethodGet {
			v, found := s.store.GetUserStatus(cred.appID, userID)
			if !found {
				writeError(w, http.StatusNotFound, "user_not_found")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"user": v})
			return
		}
		var req struct {
			Status string `json:"status"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		if req.Status != "active" && req.Status != "suspended" {
			writeError(w, http.StatusBadRequest, "status_must_be_active_or_suspended")
			return
		}
		v := UserStatus{UserID: userID, Status: req.Status, UpdatedAt: s.now()}
		s.store.PutUserStatus(cred.appID, v)
		writeJSON(w, http.StatusOK, map[string]any{"user": v})
	default:
		writeError(w, http.StatusNotFound, "not_found")
	}
}

func (s *Server) authenticate(r *http.Request) (credentialRecord, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		return credentialRecord{}, false
	}
	raw := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if raw == "" {
		return credentialRecord{}, false
	}
	h := sha256.Sum256([]byte(raw))
	id := hex.EncodeToString(h[:])
	c, ok := s.keys[id]
	if !ok {
		return credentialRecord{}, false
	}
	return c, subtle.ConstantTimeCompare(c.hash[:], h[:]) == 1
}

func (s *Server) require(w http.ResponseWriter, c credentialRecord, scope string) bool {
	if c.scopes[scope] {
		return true
	}
	writeError(w, http.StatusForbidden, "insufficient_scope")
	return false
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return false
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "trailing_json")
		return false
	}
	return true
}

func validPlatform(v string) bool {
	return v == "android" || v == "linux" || v == "windows" || v == "ios" || v == "macos" || v == "web"
}

func validReleaseURL(v string) bool {
	if len(v) == 0 || len(v) > 2048 || strings.ContainsAny(v, "\r\n") {
		return false
	}
	u, err := url.ParseRequestURI(v)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil
}

func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return time.Now().UTC().Format("20060102T150405.000000000")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

// KnownScopes returns a sorted copy for tooling and configuration validation.
func KnownScopes() []string {
	v := []string{ScopeRead, ScopeManageUsers, ScopeManageAnnouncements, ScopeManageReleases}
	sort.Strings(v)
	return v
}
