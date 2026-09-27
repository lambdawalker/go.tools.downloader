// Package store handles atomic persistence and retrieval of download sessions on disk as JSON files.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"downloader/internal/model"
)

// Store coordinates reading and writing session state to the user's filesystem.
type Store struct {
	baseDir string
	mu      sync.Mutex
}

// DefaultStore creates or opens the default store in ~/.downloader/sessions.
func DefaultStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".downloader")
	if err := os.MkdirAll(filepath.Join(dir, "sessions"), 0755); err != nil {
		return nil, fmt.Errorf("failed creating store directory: %w", err)
	}
	return &Store{baseDir: dir}, nil
}

// sessionFilePath returns the absolute JSON file path for a session name.
func (s *Store) sessionFilePath(name string) string {
	safeName := strings.ReplaceAll(name, "/", "_")
	safeName = strings.ReplaceAll(safeName, "\\", "_")
	return filepath.Join(s.baseDir, "sessions", safeName+".json")
}

// SaveSession atomically writes the serialized session state to disk using a temporary file.
func (s *Store) SaveSession(sess *model.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("failed serializing session: %w", err)
	}

	target := s.sessionFilePath(sess.Name)
	tmpFile := target + ".tmp"

	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed writing tmp session: %w", err)
	}

	if err := os.Rename(tmpFile, target); err != nil {
		return fmt.Errorf("failed committing session file: %w", err)
	}

	return nil
}

// LoadSession reads and parses a previously saved session JSON file by name.
func (s *Store) LoadSession(name string) (*model.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.sessionFilePath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("session %q not found: %w", name, err)
	}

	var sess model.Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("corrupted session file %q: %w", name, err)
	}

	return &sess, nil
}

// ListSessions scans the store directory and returns all discovered sessions.
func (s *Store) ListSessions() ([]*model.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Join(s.baseDir, "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var sessions []*model.Session
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			path := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var sess model.Session
			if err := json.Unmarshal(data, &sess); err == nil {
				sessions = append(sessions, &sess)
			}
		}
	}
	return sessions, nil
}
