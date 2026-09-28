// Package auth provides extensible authentication abstractions, persistent credential
// caching, domain-scoped security transports, master-password-encrypted vaults, and typed error handling.
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/crypto/pbkdf2"
)

const (
	vaultMagicHeader = "DLVAULT1" // 8-byte magic header
	kdfIterations    = 100_000    // PBKDF2-HMAC-SHA256 iterations
	saltLength       = 32         // 256-bit cryptographic salt
	keyLength        = 32         // AES-256 key size
)

var (
	// ErrVaultLocked is returned when attempting to access vault data before unlocking.
	ErrVaultLocked = errors.New("vault is locked; master password required")
	// ErrInvalidMasterPassword is returned when the provided master password fails decryption or integrity check.
	ErrInvalidMasterPassword = errors.New("invalid master password or corrupted vault")
	// ErrProfileNotFound is returned when an explicit profile cannot be found in the vault.
	ErrProfileNotFound = errors.New("auth profile not found in vault")
)

// Profile represents an individual authentication configuration stored securely within the vault.
type Profile struct {
	Method   MethodType        `json:"method"`
	Token    string            `json:"token,omitempty"`
	Username string            `json:"username,omitempty"`
	Password string            `json:"password,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
}

// VaultData represents the decrypted, in-memory payload of the encrypted credential vault.
type VaultData struct {
	Version        int                 `json:"version"`
	Profiles       map[string]*Profile `json:"profiles"`
	DomainDefaults map[string]string   `json:"domain_defaults"`
}

// Vault manages an encrypted AES-256-GCM credential vault protected by a master password.
type Vault struct {
	filePath string
	unlocked bool
	cachedPW string
	data     *VaultData
	mu       sync.RWMutex
}

// NewVault constructs a Vault manager targeting the specified file path.
func NewVault(filePath string) *Vault {
	return &Vault{
		filePath: filePath,
		data: &VaultData{
			Version:        1,
			Profiles:       make(map[string]*Profile),
			DomainDefaults: make(map[string]string),
		},
	}
}

// DefaultVaultPath returns the standard vault location: ~/.downloader/auth/vault.enc
func DefaultVaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed determining user home dir: %w", err)
	}
	return filepath.Join(home, ".downloader", "auth", "vault.enc"), nil
}

// DefaultVault initializes a Vault with the default path ~/.downloader/auth/vault.enc.
func DefaultVault() (*Vault, error) {
	path, err := DefaultVaultPath()
	if err != nil {
		return nil, err
	}
	return NewVault(path), nil
}

// Path returns the underlying filesystem path of the vault.
func (v *Vault) Path() string {
	return v.filePath
}

// Exists checks if the vault file exists on disk.
func (v *Vault) Exists() bool {
	info, err := os.Stat(v.filePath)
	return err == nil && !info.IsDir()
}

// IsUnlocked returns true if the vault is currently unlocked in memory.
func (v *Vault) IsUnlocked() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.unlocked
}

// Init creates a new, empty vault encrypted with the given master password.
func (v *Vault) Init(password string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if password == "" {
		return errors.New("master password cannot be empty")
	}

	v.data = &VaultData{
		Version:        1,
		Profiles:       make(map[string]*Profile),
		DomainDefaults: make(map[string]string),
	}
	v.cachedPW = password
	v.unlocked = true

	return v.saveLocked()
}

// Unlock reads the encrypted vault from disk, derives the key using the master password, and decrypts the contents.
func (v *Vault) Unlock(password string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	raw, err := os.ReadFile(v.filePath)
	if err != nil {
		return fmt.Errorf("failed reading vault file: %w", err)
	}

	// File structure:
	// [0..8]   Magic ("DLVAULT1")
	// [8..40]  Salt (32 bytes)
	// [40..52] Nonce (12 bytes)
	// [52..]   Ciphertext + Tag (min 16 bytes)
	const minLen = len(vaultMagicHeader) + saltLength + 12 + 16
	if len(raw) < minLen {
		return errors.New("vault file is truncated or corrupted")
	}

	magic := string(raw[:len(vaultMagicHeader)])
	if magic != vaultMagicHeader {
		return errors.New("unrecognized vault header; not a valid downloader vault")
	}

	saltOffset := len(vaultMagicHeader)
	nonceOffset := saltOffset + saltLength
	cipherOffset := nonceOffset + 12

	salt := raw[saltOffset:nonceOffset]
	nonce := raw[nonceOffset:cipherOffset]
	ciphertext := raw[cipherOffset:]

	key := pbkdf2.Key([]byte(password), salt, kdfIterations, keyLength, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed creating gcm: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(vaultMagicHeader))
	if err != nil {
		return ErrInvalidMasterPassword
	}

	var data VaultData
	if err := json.Unmarshal(plaintext, &data); err != nil {
		return fmt.Errorf("failed parsing decrypted vault payload: %w", err)
	}

	if data.Profiles == nil {
		data.Profiles = make(map[string]*Profile)
	}
	if data.DomainDefaults == nil {
		data.DomainDefaults = make(map[string]string)
	}

	v.data = &data
	v.cachedPW = password
	v.unlocked = true
	return nil
}

// Save re-encrypts the in-memory vault and atomically persists it to disk.
func (v *Vault) Save() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.unlocked {
		return ErrVaultLocked
	}
	return v.saveLocked()
}

// saveLocked persists the vault assuming the write lock is already held.
func (v *Vault) saveLocked() error {
	if v.cachedPW == "" {
		return ErrVaultLocked
	}

	plaintext, err := json.Marshal(v.data)
	if err != nil {
		return fmt.Errorf("failed marshaling vault data: %w", err)
	}

	// 1. Generate fresh cryptographic salt
	salt := make([]byte, saltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("failed generating random salt: %w", err)
	}

	// 2. Derive key via PBKDF2
	key := pbkdf2.Key([]byte(v.cachedPW), salt, kdfIterations, keyLength, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed creating gcm: %w", err)
	}

	// 3. Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed generating random nonce: %w", err)
	}

	// 4. Encrypt with AES-GCM (bind magic header as authenticated data)
	ciphertext := gcm.Seal(nil, nonce, plaintext, []byte(vaultMagicHeader))

	// 5. Assemble encrypted payload
	buf := make([]byte, 0, len(vaultMagicHeader)+len(salt)+len(nonce)+len(ciphertext))
	buf = append(buf, []byte(vaultMagicHeader)...)
	buf = append(buf, salt...)
	buf = append(buf, nonce...)
	buf = append(buf, ciphertext...)

	// 6. Ensure directory exists with 0700 permissions
	dir := filepath.Dir(v.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed creating vault directory: %w", err)
	}

	// 7. Atomic file write
	tmpFile := fmt.Sprintf("%s.tmp.%d", v.filePath, os.Getpid())
	if err := os.WriteFile(tmpFile, buf, 0600); err != nil {
		return fmt.Errorf("failed writing temporary vault file: %w", err)
	}

	if err := os.Rename(tmpFile, v.filePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed replacing vault file: %w", err)
	}

	return nil
}

// ChangePassword re-encrypts the vault with a new master password.
func (v *Vault) ChangePassword(newPassword string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.unlocked {
		return ErrVaultLocked
	}
	if newPassword == "" {
		return errors.New("new master password cannot be empty")
	}

	v.cachedPW = newPassword
	return v.saveLocked()
}

// SetProfile adds or updates a named authentication profile in the vault.
func (v *Vault) SetProfile(name string, profile *Profile) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.unlocked {
		return ErrVaultLocked
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("profile name cannot be empty")
	}

	v.data.Profiles[name] = profile
	return v.saveLocked()
}

// GetProfile retrieves a profile by name from the unlocked vault.
func (v *Vault) GetProfile(name string) (*Profile, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if !v.unlocked || v.data == nil {
		return nil, false
	}

	p, ok := v.data.Profiles[strings.TrimSpace(name)]
	return p, ok
}

// DeleteProfile removes a named profile from the vault.
func (v *Vault) DeleteProfile(name string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.unlocked || v.data == nil {
		return false
	}

	name = strings.TrimSpace(name)
	if _, ok := v.data.Profiles[name]; ok {
		delete(v.data.Profiles, name)
		_ = v.saveLocked()
		return true
	}
	return false
}

// ListProfiles returns sorted names of all profiles defined in the vault.
func (v *Vault) ListProfiles() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if !v.unlocked || v.data == nil {
		return nil
	}

	names := make([]string, 0, len(v.data.Profiles))
	for name := range v.data.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SetDomainDefault maps a domain or hostname pattern to a default profile name.
func (v *Vault) SetDomainDefault(domain, profileName string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.unlocked {
		return ErrVaultLocked
	}

	domain = NormalizeHostname(domain)
	v.data.DomainDefaults[domain] = strings.TrimSpace(profileName)
	return v.saveLocked()
}

// GetDomainDefault finds the profile mapped to a domain, supporting wildcards.
func (v *Vault) GetDomainDefault(domain string) (string, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if !v.unlocked || v.data == nil {
		return "", false
	}

	norm := NormalizeHostname(domain)
	if p, ok := v.data.DomainDefaults[norm]; ok {
		return p, true
	}

	for pattern, p := range v.data.DomainDefaults {
		if strings.HasPrefix(pattern, "*.") {
			suffix := strings.TrimPrefix(pattern, "*.")
			if strings.HasSuffix(norm, suffix) {
				return p, true
			}
		}
	}

	return "", false
}

// ProfileToStrategy converts a vault Profile into an executable auth Strategy.
func ProfileToStrategy(p *Profile) Strategy {
	if p == nil {
		return NewAnonymousStrategy()
	}

	switch p.Method {
	case MethodBearer:
		return NewBearerStrategy(p.Token)
	case MethodBasic:
		return NewBasicStrategy(p.Username, p.Password)
	case MethodHeader:
		return NewHeaderStrategy(p.Headers)
	default:
		return NewAnonymousStrategy()
	}
}

// ResolveStrategy resolves the appropriate auth Strategy for a job URL or explicit profile name.
func (v *Vault) ResolveStrategy(explicitProfile, targetURL string) (Strategy, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// 1. Explicit profile specified in URL list (e.g. auth=my-profile)
	if explicitProfile != "" {
		if !v.unlocked {
			return nil, ErrVaultLocked
		}
		p, ok := v.data.Profiles[explicitProfile]
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrProfileNotFound, explicitProfile)
		}
		return ProfileToStrategy(p), nil
	}

	// 2. Domain default mapping in vault
	if v.unlocked && targetURL != "" {
		if u, err := url.Parse(targetURL); err == nil {
			if profName, ok := v.GetDomainDefault(u.Hostname()); ok {
				if p, found := v.data.Profiles[profName]; found {
					return ProfileToStrategy(p), nil
				}
			}
		}
	}

	// 3. Fallback to unauthenticated access
	return NewAnonymousStrategy(), nil
}
