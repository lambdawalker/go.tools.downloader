package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStrategies(t *testing.T) {
	// 1. Anonymous Strategy
	anon := NewAnonymousStrategy()
	if anon.Type() != MethodNone {
		t.Fatalf("expected MethodNone, got %s", anon.Type())
	}
	req, _ := http.NewRequest("GET", "https://example.com/test", nil)
	if err := anon.Apply(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Header.Get("Authorization") != "" {
		t.Fatalf("expected no Authorization header")
	}

	// 2. Bearer Strategy
	bearer := NewBearerStrategy("secret-token-123")
	if bearer.Type() != MethodBearer {
		t.Fatalf("expected MethodBearer, got %s", bearer.Type())
	}
	req, _ = http.NewRequest("GET", "https://example.com/test", nil)
	if err := bearer.Apply(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Header.Get("Authorization") != "Bearer secret-token-123" {
		t.Fatalf("expected Bearer token, got %s", req.Header.Get("Authorization"))
	}

	// 3. Bearer with env var
	t.Setenv("TEST_AUTH_TOKEN", "env-token-xyz")
	bearerEnv := NewBearerStrategy("env:TEST_AUTH_TOKEN")
	req, _ = http.NewRequest("GET", "https://example.com/test", nil)
	if err := bearerEnv.Apply(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Header.Get("Authorization") != "Bearer env-token-xyz" {
		t.Fatalf("expected Bearer from env, got %s", req.Header.Get("Authorization"))
	}

	// 4. Basic Auth Strategy
	basic := NewBasicStrategy("myuser", "mypass")
	if basic.Type() != MethodBasic {
		t.Fatalf("expected MethodBasic, got %s", basic.Type())
	}
	req, _ = http.NewRequest("GET", "https://example.com/test", nil)
	if err := basic.Apply(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	u, p, ok := req.BasicAuth()
	if !ok || u != "myuser" || p != "mypass" {
		t.Fatalf("expected basic auth myuser:mypass, got %s:%s (ok=%v)", u, p, ok)
	}

	// 5. Custom Header Strategy
	headers := map[string]string{
		"X-API-Key":     "key-999",
		"Private-Token": "gl-token-abc",
	}
	hdrStrat := NewHeaderStrategy(headers)
	if hdrStrat.Type() != MethodHeader {
		t.Fatalf("expected MethodHeader, got %s", hdrStrat.Type())
	}
	req, _ = http.NewRequest("GET", "https://example.com/test", nil)
	if err := hdrStrat.Apply(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Header.Get("X-API-Key") != "key-999" || req.Header.Get("Private-Token") != "gl-token-abc" {
		t.Fatalf("header strategy failed to set headers: %v", req.Header)
	}
}

func TestCrossDomainRedirectStripping(t *testing.T) {
	// Mock cross-domain redirect server:
	// Server 2 (CDN / S3 target): checks that NO auth headers are sent!
	var receivedAuthHeader string
	var receivedAPIKeyHeader string

	cdnServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		receivedAPIKeyHeader = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("CDN content"))
	}))
	defer cdnServer.Close()

	// Server 1 (Primary target): redirects to cdnServer
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Server 1 must receive the auth header!
		if r.Header.Get("Authorization") != "Bearer secret-123" {
			t.Errorf("primary server did not receive auth header: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-API-Key") != "my-key" {
			t.Errorf("primary server did not receive X-API-Key header: %s", r.Header.Get("X-API-Key"))
		}
		http.Redirect(w, r, cdnServer.URL, http.StatusFound)
	}))
	defer primaryServer.Close()

	// Configure strategy with Bearer and custom header
	strat := NewHeaderStrategy(map[string]string{
		"Authorization": "Bearer secret-123",
		"X-API-Key":     "my-key",
	})

	client := NewScopedHTTPClient(primaryServer.Listener.Addr().String(), strat, 5*time.Second)

	resp, err := client.Get(primaryServer.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Verify that the CDN server did NOT receive any auth headers!
	if receivedAuthHeader != "" {
		t.Fatalf("SECURITY VIOLATION: Authorization header leaked across redirect to %s: %s", cdnServer.URL, receivedAuthHeader)
	}
	if receivedAPIKeyHeader != "" {
		t.Fatalf("SECURITY VIOLATION: X-API-Key header leaked across redirect to %s: %s", cdnServer.URL, receivedAPIKeyHeader)
	}
}

func TestStorePersistenceAndDomainMatching(t *testing.T) {
	tmpDir := t.TempDir()

	cfgPath := filepath.Join(tmpDir, "config.json")
	store, err := NewStore(cfgPath)
	if err != nil {
		t.Fatalf("failed initializing store: %v", err)
	}

	// 1. Add entry for domain
	err = store.Set("api.github.com", &Entry{
		Method: MethodBearer,
		Token:  "ghp_testtoken",
	})
	if err != nil {
		t.Fatalf("failed setting entry: %v", err)
	}

	// 2. Query domain with port and uppercase
	entry, found := store.Get("API.GITHUB.COM:443")
	if !found || entry == nil {
		t.Fatalf("failed finding normalized domain entry")
	}
	if entry.Method != MethodBearer || entry.Token != "ghp_testtoken" {
		t.Fatalf("unexpected entry: %+v", entry)
	}

	// 3. Add wildcard domain
	err = store.Set("*.company.internal", &Entry{
		Method:   MethodBasic,
		Username: "alice",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("failed setting wildcard entry: %v", err)
	}

	entryWild, found := store.Get("downloads.company.internal")
	if !found || entryWild == nil {
		t.Fatalf("failed finding wildcard domain entry")
	}
	if entryWild.Username != "alice" {
		t.Fatalf("unexpected wildcard match result: %+v", entryWild)
	}

	// 4. Reload from disk
	store2, err := NewStore(cfgPath)
	if err != nil {
		t.Fatalf("failed reloading store: %v", err)
	}
	entryReload, found := store2.Get("api.github.com")
	if !found || entryReload.Token != "ghp_testtoken" {
		t.Fatalf("failed reloading entry from disk: %+v", entryReload)
	}
}

func TestManagerAndPrompter(t *testing.T) {
	tmpDir := t.TempDir()

	store, _ := NewStore(filepath.Join(tmpDir, "config.json"))

	promptCount := 0
	prompter := NewCallbackPrompter(func(_ string) (*Entry, error) {
		promptCount++
		return &Entry{
			Method: MethodBearer,
			Token:  "token_from_prompt",
		}, nil
	})

	mgr := NewManager(store, prompter)

	// First query: domain not in cache -> prompter called
	strat1, err := mgr.ResolveDomain("example.org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strat1.Type() != MethodBearer {
		t.Fatalf("expected MethodBearer, got %s", strat1.Type())
	}
	if promptCount != 1 {
		t.Fatalf("expected prompter to be called once, got %d", promptCount)
	}

	// Second query for same domain: should hit cache immediately and NOT call prompter!
	strat2, err := mgr.ResolveDomain("example.org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strat2.Type() != MethodBearer {
		t.Fatalf("expected MethodBearer, got %s", strat2.Type())
	}
	if promptCount != 1 {
		t.Fatalf("expected prompter to NOT be called again, but promptCount=%d", promptCount)
	}
}

func TestCustomTerminalPrompter(t *testing.T) {
	// Simulate user entering "2" then "my-token\n"
	input := strings.NewReader("2\nmy-token\n")
	output := &bytes.Buffer{}
	prompter := NewCustomTerminalPrompter(input, output)

	entry, err := prompter.PromptAuth("github.com")
	if err != nil {
		t.Fatalf("PromptAuth failed: %v", err)
	}
	if entry.Method != MethodBearer || entry.Token != "my-token" {
		t.Fatalf("unexpected entry: %+v", entry)
	}

	// Test NonInteractivePrompter
	nonInteractive := NewNonInteractivePrompter()
	entryAnon, err := nonInteractive.PromptAuth("any.com")
	if err != nil {
		t.Fatalf("NonInteractivePrompter failed: %v", err)
	}
	if entryAnon.Method != MethodNone {
		t.Fatalf("expected MethodNone, got %s", entryAnon.Method)
	}

	if ErrPromptAborted.Error() != "authentication prompt canceled by user" {
		t.Fatalf("unexpected ErrPromptAborted: %v", ErrPromptAborted)
	}
}

func TestTypedAuthErrors(t *testing.T) {
	// 401 Unauthorized
	err401 := MapStatusCode(http.StatusUnauthorized, "api.example.com", "https://api.example.com/file", nil, nil)
	if !IsAuthError(err401) {
		t.Fatalf("expected IsAuthError true for 401")
	}
	if !IsUnauthorized(err401) {
		t.Fatalf("expected IsUnauthorized true for 401")
	}

	// 403 Forbidden
	err403 := MapStatusCode(http.StatusForbidden, "api.example.com", "https://api.example.com/file", nil, nil)
	if !IsForbidden(err403) {
		t.Fatalf("expected IsForbidden true for 403")
	}

	// 429 Rate Limited with Retry-After
	hdr := http.Header{}
	hdr.Set("Retry-After", "120")
	err429 := MapStatusCode(http.StatusTooManyRequests, "api.example.com", "https://api.example.com/file", hdr, nil)
	if !IsRateLimited(err429) {
		t.Fatalf("expected IsRateLimited true for 429")
	}
	authErr, _ := AsAuthError(err429)
	if authErr.RetryAfter != 120*time.Second {
		t.Fatalf("expected RetryAfter 120s, got %s", authErr.RetryAfter)
	}

	// 200 OK -> nil
	if MapStatusCode(http.StatusOK, "example.com", "https://example.com", nil, nil) != nil {
		t.Fatalf("expected nil for 200 OK")
	}
}
