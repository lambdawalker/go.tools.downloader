package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Entry defines the cached authentication parameters for a specific domain or site.
type Entry struct {
	Method   MethodType        `json:"method"`             // "none", "bearer", "basic", "header"
	Token    string            `json:"token,omitempty"`    // for bearer token (can be env:VAR or $VAR)
	Username string            `json:"username,omitempty"` // for basic auth username
	Password string            `json:"password,omitempty"` // for basic auth password
	Headers  map[string]string `json:"headers,omitempty"`  // for custom headers
}

// Config represents the root structure persisted in the auth cache file.
type Config struct {
	Version int               `json:"version"`
	Entries map[string]*Entry `json:"entries"`
}

// Store provides thread-safe persistent caching for domain credentials.
type Store struct {
	filePath string
	mu       sync.RWMutex
	config   Config
}

// DefaultAuthFilePath resolves the default credential cache file under ~/.downloader/auth/config.json
// or ~/.downloader/auth if a regular file already exists at that path.
func DefaultAuthFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	authPath := filepath.Join(home, ".downloader", "auth")
	info, statErr := os.Stat(authPath)
	if statErr == nil && !info.IsDir() {
		// ~/.downloader/auth exists as a regular file
		return authPath, nil
	}

	// Otherwise use ~/.downloader/auth/config.json
	if err := os.MkdirAll(authPath, 0700); err != nil {
		return "", fmt.Errorf("failed creating auth directory %q: %w", authPath, err)
	}
	return filepath.Join(authPath, "config.json"), nil
}

// DefaultStore initializes a Store using the default path ~/.downloader/auth.
func DefaultStore() (*Store, error) {
	path, err := DefaultAuthFilePath()
	if err != nil {
		return nil, err
	}
	return NewStore(path)
}

// NewStore initializes a Store with the specified JSON configuration file path.
func NewStore(filePath string) (*Store, error) {
	st := &Store{
		filePath: filePath,
		config: Config{
			Version: 1,
			Entries: make(map[string]*Entry),
		},
	}

	if err := st.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return st, nil
}

// load reads and parses the JSON cache from disk.
func (s *Store) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed parsing auth cache file %q: %w", s.filePath, err)
	}

	if cfg.Entries == nil {
		cfg.Entries = make(map[string]*Entry)
	}
	s.config = cfg
	return nil
}

// save atomically persists the in-memory cache to disk.
func (s *Store) save() error {
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed creating auth directory %q: %w", dir, err)
	}

	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed serializing auth config: %w", err)
	}

	tmpFile := s.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("failed writing tmp auth config: %w", err)
	}

	if err := os.Rename(tmpFile, s.filePath); err != nil {
		return fmt.Errorf("failed committing auth cache %q: %w", s.filePath, err)
	}

	return nil
}

// Get finds an authentication entry for domain, trying exact match, normalized host, and wildcard suffixes.
func (s *Store) Get(domain string) (*Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	norm := NormalizeHostname(domain)

	// 1. Exact match on raw input
	if entry, ok := s.config.Entries[domain]; ok {
		return entry, true
	}

	// 2. Normalized hostname match
	if entry, ok := s.config.Entries[norm]; ok {
		return entry, true
	}

	// 3. Case-insensitive search & wildcard patterns (e.g. *.github.com)
	for key, entry := range s.config.Entries {
		keyNorm := NormalizeHostname(key)
		if strings.EqualFold(keyNorm, norm) {
			return entry, true
		}
		if strings.HasPrefix(key, "*.") {
			suffix := strings.TrimPrefix(key, "*.")
			if strings.HasSuffix(norm, suffix) {
				return entry, true
			}
		}
	}

	return nil, false
}

// Set adds or updates an authentication entry for domain and persists it immediately.
func (s *Store) Set(domain string, entry *Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	norm := NormalizeHostname(domain)
	if norm == "" {
		norm = domain
	}

	s.config.Entries[norm] = entry
	return s.save()
}

// ListEntries returns a map of all configured domains and their entry parameters.
func (s *Store) ListEntries() map[string]*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make(map[string]*Entry, len(s.config.Entries))
	for k, v := range s.config.Entries {
		res[k] = v
	}
	return res
}

// ToStrategy converts an Entry into a corresponding Strategy instance.
func ToStrategy(entry *Entry) (Strategy, error) {
	if entry == nil {
		return NewAnonymousStrategy(), nil
	}

	switch entry.Method {
	case MethodNone, "":
		return NewAnonymousStrategy(), nil
	case MethodBearer:
		return NewBearerStrategy(entry.Token), nil
	case MethodBasic:
		return NewBasicStrategy(entry.Username, entry.Password), nil
	case MethodHeader:
		return NewHeaderStrategy(entry.Headers), nil
	default:
		return nil, fmt.Errorf("unsupported auth method: %s", entry.Method)
	}
}
