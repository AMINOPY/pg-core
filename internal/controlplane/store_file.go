package controlplane

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type persistedState struct {
	Announcements map[string]Announcement            `json:"announcements"`
	Releases      map[string]map[string]Release      `json:"releases"`
	Users         map[string]map[string]UserStatus   `json:"users"`
}

// FileStore is a single-process durable store. It atomically replaces a JSON
// state file on writes and uses restrictive file permissions. For multi-instance
// deployments use a transactional database-backed Store implementation instead.
type FileStore struct {
	mu    sync.RWMutex
	path  string
	state persistedState
}

// NewFileStore opens or creates a state file with owner-only permissions.
func NewFileStore(path string) (*FileStore, error) {
	if path == "" {
		return nil, errors.New("controlplane: state file path is required")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	s := &FileStore{path: path, state: emptyState()}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := s.persistLocked(s.state); err != nil {
			return nil, err
		}
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s.state); err != nil {
		return nil, errors.New("controlplane: invalid state file")
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, errors.New("controlplane: trailing data in state file")
	}
	normalizeState(&s.state)
	if err := os.Chmod(path, 0600); err != nil {
		return nil, err
	}
	return s, nil
}

func emptyState() persistedState {
	return persistedState{
		Announcements: make(map[string]Announcement),
		Releases:      make(map[string]map[string]Release),
		Users:         make(map[string]map[string]UserStatus),
	}
}

func normalizeState(s *persistedState) {
	if s.Announcements == nil {
		s.Announcements = make(map[string]Announcement)
	}
	if s.Releases == nil {
		s.Releases = make(map[string]map[string]Release)
	}
	if s.Users == nil {
		s.Users = make(map[string]map[string]UserStatus)
	}
}

func cloneState(src persistedState) persistedState {
	dst := emptyState()
	for k, v := range src.Announcements {
		dst.Announcements[k] = v
	}
	for app, releases := range src.Releases {
		dst.Releases[app] = make(map[string]Release, len(releases))
		for platform, v := range releases {
			dst.Releases[app][platform] = v
		}
	}
	for app, users := range src.Users {
		dst.Users[app] = make(map[string]UserStatus, len(users))
		for userID, v := range users {
			dst.Users[app][userID] = v
		}
	}
	return dst
}

func (s *FileStore) persistLocked(next persistedState) error {
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".pgcore-controlplane-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return err
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

func (s *FileStore) GetAnnouncement(appID string, now time.Time) (Announcement, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.state.Announcements[appID]
	if ok && !v.ExpiresAt.IsZero() && !now.Before(v.ExpiresAt) {
		return Announcement{}, false, nil
	}
	return v, ok, nil
}

func (s *FileStore) PutAnnouncement(appID string, v Announcement) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneState(s.state)
	next.Announcements[appID] = v
	if err := s.persistLocked(next); err != nil {
		return err
	}
	s.state = next
	return nil
}

func (s *FileStore) GetRelease(appID, platform string) (Release, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.state.Releases[appID][platform]
	return v, ok, nil
}

func (s *FileStore) PutRelease(appID string, v Release) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneState(s.state)
	if next.Releases[appID] == nil {
		next.Releases[appID] = make(map[string]Release)
	}
	next.Releases[appID][v.Platform] = v
	if err := s.persistLocked(next); err != nil {
		return err
	}
	s.state = next
	return nil
}

func (s *FileStore) GetUserStatus(appID, userID string) (UserStatus, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.state.Users[appID][userID]
	return v, ok, nil
}

func (s *FileStore) PutUserStatus(appID string, v UserStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneState(s.state)
	if next.Users[appID] == nil {
		next.Users[appID] = make(map[string]UserStatus)
	}
	next.Users[appID][v.UserID] = v
	if err := s.persistLocked(next); err != nil {
		return err
	}
	s.state = next
	return nil
}
