package auth

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestVaultInitAndUnlock(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vault_test_*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "vault.enc")
	v := NewVault(vaultPath)

	if v.Exists() {
		t.Fatalf("vault should not exist before init")
	}

	masterPW := "SuperSecret123!"

	// 1. Initialize vault
	if err := v.Init(masterPW); err != nil {
		t.Fatalf("failed initializing vault: %v", err)
	}

	if !v.Exists() {
		t.Fatalf("vault should exist after init")
	}
	if !v.IsUnlocked() {
		t.Fatalf("vault should be unlocked after init")
	}

	// 2. Add profile
	p := &Profile{
		Method: MethodBearer,
		Token:  "ghp_test_token_999",
	}
	if err := v.SetProfile("github-ci", p); err != nil {
		t.Fatalf("failed setting profile: %v", err)
	}

	// 3. Unlock with new Vault instance (correct password)
	v2 := NewVault(vaultPath)
	if v2.IsUnlocked() {
		t.Fatalf("new vault instance should start locked")
	}

	if err := v2.Unlock(masterPW); err != nil {
		t.Fatalf("failed unlocking vault with correct password: %v", err)
	}
	if !v2.IsUnlocked() {
		t.Fatalf("vault should be unlocked after Unlock")
	}

	retrieved, ok := v2.GetProfile("github-ci")
	if !ok || retrieved == nil {
		t.Fatalf("failed retrieving profile from unlocked vault")
	}
	if retrieved.Token != "ghp_test_token_999" {
		t.Fatalf("token mismatch: got %s", retrieved.Token)
	}

	// 4. Unlock with incorrect password
	v3 := NewVault(vaultPath)
	err = v3.Unlock("WrongPassword!")
	if err == nil {
		t.Fatalf("expected error unlocking with wrong password, got nil")
	}
	if !errors.Is(err, ErrInvalidMasterPassword) {
		t.Fatalf("expected ErrInvalidMasterPassword, got: %v", err)
	}
}

func TestVaultTamperDetection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vault_tamper_*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "vault.enc")
	v := NewVault(vaultPath)
	_ = v.Init("MyPassword")
	_ = v.SetProfile("test", &Profile{Method: MethodBearer, Token: "abc"})

	// Read ciphertext and tamper with a byte in the encrypted block
	data, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatalf("failed reading vault file: %v", err)
	}

	// Tamper with the last byte
	data[len(data)-1] ^= 0xFF
	if err := os.WriteFile(vaultPath, data, 0600); err != nil {
		t.Fatalf("failed writing tampered vault: %v", err)
	}

	v2 := NewVault(vaultPath)
	err = v2.Unlock("MyPassword")
	if err == nil {
		t.Fatalf("expected decryption error due to tampering, got nil")
	}
	if !errors.Is(err, ErrInvalidMasterPassword) {
		t.Fatalf("expected ErrInvalidMasterPassword on tampered file, got: %v", err)
	}
}

func TestVaultChangePassword(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vault_pw_*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "vault.enc")
	v := NewVault(vaultPath)
	_ = v.Init("InitialPassword1")
	_ = v.SetProfile("nexus", &Profile{
		Method:   MethodBasic,
		Username: "admin",
		Password: "secret-admin-pass",
	})

	// Change password
	if err := v.ChangePassword("NewMasterPassword2"); err != nil {
		t.Fatalf("failed changing password: %v", err)
	}

	// Try unlocking with old password -> should fail
	vOld := NewVault(vaultPath)
	if err := vOld.Unlock("InitialPassword1"); err == nil {
		t.Fatalf("unlocking with old password should fail after change")
	}

	// Try unlocking with new password -> should succeed
	vNew := NewVault(vaultPath)
	if err := vNew.Unlock("NewMasterPassword2"); err != nil {
		t.Fatalf("failed unlocking with new password: %v", err)
	}

	p, ok := vNew.GetProfile("nexus")
	if !ok || p.Password != "secret-admin-pass" {
		t.Fatalf("profile not found or damaged after password change: %+v", p)
	}
}

func TestVaultProfileAndDomainResolutions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vault_resolve_*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "vault.enc")
	v := NewVault(vaultPath)
	_ = v.Init("Pass123")

	_ = v.SetProfile("gh", &Profile{
		Method: MethodBearer,
		Token:  "ghp_token",
	})
	_ = v.SetProfile("custom-header", &Profile{
		Method: MethodHeader,
		Headers: map[string]string{
			"X-API-Key": "my-key-999",
		},
	})
	_ = v.SetDomainDefault("api.github.com", "gh")

	// 1. Resolve explicit profile "custom-header"
	strat, err := v.ResolveStrategy("custom-header", "https://api.github.com/anything")
	if err != nil {
		t.Fatalf("unexpected error resolving explicit profile: %v", err)
	}
	if strat.Type() != MethodHeader {
		t.Fatalf("expected MethodHeader, got %s", strat.Type())
	}
	req, _ := http.NewRequest("GET", "https://api.github.com", nil)
	_ = strat.Apply(req)
	if req.Header.Get("X-API-Key") != "my-key-999" {
		t.Fatalf("expected header X-API-Key: my-key-999")
	}

	// 2. Resolve domain default (no explicit profile)
	strat2, err := v.ResolveStrategy("", "https://api.github.com/repos/org/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strat2.Type() != MethodBearer {
		t.Fatalf("expected MethodBearer for domain default, got %s", strat2.Type())
	}

	// 3. Resolve unrecognized domain with no explicit profile -> Anonymous
	strat3, err := v.ResolveStrategy("", "https://public.cdn.org/installer.iso")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strat3.Type() != MethodNone {
		t.Fatalf("expected MethodNone, got %s", strat3.Type())
	}

	// 4. Resolve non-existent explicit profile -> error
	_, err = v.ResolveStrategy("does-not-exist", "https://any.com")
	if err == nil || !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("expected ErrProfileNotFound, got: %v", err)
	}
}
