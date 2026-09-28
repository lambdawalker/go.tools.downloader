// Package auth provides extensible authentication abstractions, persistent credential
// caching, domain-scoped security transports, and typed error handling.
package auth

import (
	"encoding/base64"
	"net/http"
	"os"
	"sort"
	"strings"
)

// MethodType identifies the authentication mechanism.
type MethodType string

const (
	// MethodNone indicates unauthenticated (anonymous) access.
	MethodNone MethodType = "none"
	// MethodBearer indicates Bearer token authentication via HTTP Authorization header.
	MethodBearer MethodType = "bearer"
	// MethodBasic indicates HTTP Basic Authentication.
	MethodBasic MethodType = "basic"
	// MethodHeader indicates custom HTTP headers (e.g. API keys, personal access tokens).
	MethodHeader MethodType = "header"
)

// Strategy represents an authentication mechanism that injects credentials into HTTP requests.
type Strategy interface {
	// Type returns the method identifier for this strategy.
	Type() MethodType
	// Apply injects authentication credentials into the provided HTTP request.
	Apply(req *http.Request) error
	// Headers returns the names of the HTTP headers populated by this strategy.
	// Used by scoped transports to strip sensitive headers on cross-domain redirects.
	Headers() []string
}

// ResolveValue resolves a credential string. If the value starts with "env:" or "$",
// the corresponding environment variable is read. Otherwise, the raw string is returned.
func ResolveValue(val string) string {
	val = strings.TrimSpace(val)
	if strings.HasPrefix(val, "env:") {
		envKey := strings.TrimPrefix(val, "env:")
		return os.Getenv(strings.TrimSpace(envKey))
	}
	if strings.HasPrefix(val, "$") {
		envKey := strings.TrimPrefix(val, "$")
		return os.Getenv(strings.TrimSpace(envKey))
	}
	return val
}

// AnonymousStrategy implements unauthenticated access.
type AnonymousStrategy struct{}

// NewAnonymousStrategy returns a Strategy that adds no authentication headers.
func NewAnonymousStrategy() Strategy {
	return &AnonymousStrategy{}
}

// Type returns MethodNone.
func (s *AnonymousStrategy) Type() MethodType {
	return MethodNone
}

// Apply does not modify the request.
func (s *AnonymousStrategy) Apply(req *http.Request) error {
	return nil
}

// Headers returns nil as no headers are attached.
func (s *AnonymousStrategy) Headers() []string {
	return nil
}

// BearerStrategy injects an HTTP Authorization Bearer token.
type BearerStrategy struct {
	token string
}

// NewBearerStrategy creates a BearerStrategy using the provided token or environment variable reference.
func NewBearerStrategy(token string) *BearerStrategy {
	return &BearerStrategy{token: token}
}

// Type returns MethodBearer.
func (s *BearerStrategy) Type() MethodType {
	return MethodBearer
}

// Apply sets the Authorization header to "Bearer <resolved_token>".
func (s *BearerStrategy) Apply(req *http.Request) error {
	resolved := ResolveValue(s.token)
	if resolved == "" {
		return &AuthError{
			Kind:    KindInvalidCredentials,
			Message: "Bearer token is empty or unresolved environment variable",
		}
	}
	req.Header.Set("Authorization", "Bearer "+resolved)
	return nil
}

// Headers returns []string{"Authorization"}.
func (s *BearerStrategy) Headers() []string {
	return []string{"Authorization"}
}

// BasicStrategy injects HTTP Basic authentication credentials.
type BasicStrategy struct {
	username string
	password string
}

// NewBasicStrategy creates a BasicStrategy using the given username and password (or environment references).
func NewBasicStrategy(username, password string) *BasicStrategy {
	return &BasicStrategy{
		username: username,
		password: password,
	}
}

// Type returns MethodBasic.
func (s *BasicStrategy) Type() MethodType {
	return MethodBasic
}

// Apply sets the Authorization header with base64-encoded basic credentials.
func (s *BasicStrategy) Apply(req *http.Request) error {
	user := ResolveValue(s.username)
	pass := ResolveValue(s.password)
	auth := user + ":" + pass
	encoded := base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Set("Authorization", "Basic "+encoded)
	return nil
}

// Headers returns []string{"Authorization"}.
func (s *BasicStrategy) Headers() []string {
	return []string{"Authorization"}
}

// HeaderStrategy injects arbitrary custom HTTP headers.
type HeaderStrategy struct {
	headers map[string]string
}

// NewHeaderStrategy constructs a HeaderStrategy with key-value header pairs.
func NewHeaderStrategy(headers map[string]string) *HeaderStrategy {
	h := make(map[string]string, len(headers))
	for k, v := range headers {
		h[k] = v
	}
	return &HeaderStrategy{headers: h}
}

// Type returns MethodHeader.
func (s *HeaderStrategy) Type() MethodType {
	return MethodHeader
}

// Apply sets all custom headers onto the request.
func (s *HeaderStrategy) Apply(req *http.Request) error {
	for k, v := range s.headers {
		resolved := ResolveValue(v)
		req.Header.Set(k, resolved)
	}
	return nil
}

// Headers returns the list of header keys configured in this strategy.
func (s *HeaderStrategy) Headers() []string {
	keys := make([]string, 0, len(s.headers))
	for k := range s.headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
