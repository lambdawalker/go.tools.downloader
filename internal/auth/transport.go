package auth

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// NormalizeHostname extracts the lowercase hostname without port from a host string.
func NormalizeHostname(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

// NormalizeHostPort normalizes a host string (e.g. "api.github.com:443" -> "api.github.com").
// Non-standard ports (such as "127.0.0.1:8080" or "localhost:5000") are preserved so that
// different local test services or explicit ports are distinguished.
func NormalizeHostPort(rawHost, scheme string) string {
	rawHost = strings.TrimSpace(strings.ToLower(rawHost))
	host, port, err := net.SplitHostPort(rawHost)
	if err != nil {
		host = rawHost
		port = ""
	}

	// Normalize default HTTP/HTTPS ports to empty
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		port = ""
	} else if port == "443" || port == "80" {
		port = ""
	}

	if port == "" {
		return host
	}
	return net.JoinHostPort(host, port)
}

// IsSameDomain compares two host strings and returns true if their normalized host/ports match.
func IsSameDomain(hostA, hostB string) bool {
	normA := NormalizeHostPort(hostA, "")
	normB := NormalizeHostPort(hostB, "")
	if normA == "" || normB == "" {
		return false
	}
	return strings.EqualFold(normA, normB)
}

// ScopedTransport is an http.RoundTripper that applies authentication headers
// exclusively to the primary target domain and strips sensitive credentials
// on cross-domain redirects (such as Amazon S3, Cloudflare, or presigned CDN URLs).
type ScopedTransport struct {
	Base       http.RoundTripper
	TargetHost string
	Strategy   Strategy
}

// RoundTrip executes a single HTTP transaction while enforcing domain scoping for credentials.
func (st *ScopedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := st.Base
	if base == nil {
		base = http.DefaultTransport
	}

	clonedReq := req.Clone(req.Context())
	if clonedReq.Header == nil {
		clonedReq.Header = make(http.Header)
	}

	targetNorm := NormalizeHostPort(st.TargetHost, "")
	currentNorm := NormalizeHostPort(req.URL.Host, req.URL.Scheme)

	// If request is directed to the primary target domain, apply auth strategy
	if targetNorm != "" && strings.EqualFold(targetNorm, currentNorm) {
		if st.Strategy != nil {
			if err := st.Strategy.Apply(clonedReq); err != nil {
				return nil, err
			}
		}
	} else {
		// Cross-domain request (e.g. redirect to S3, Cloudflare CDN, different port/host):
		// Strictly strip all authentication headers to prevent storage providers from rejecting
		// requests due to conflicting or invalid authorization signatures.
		clonedReq.Header.Del("Authorization")
		clonedReq.Header.Del("Proxy-Authorization")
		clonedReq.Header.Del("Cookie")

		if st.Strategy != nil {
			for _, h := range st.Strategy.Headers() {
				clonedReq.Header.Del(h)
			}
		}

		// Also remove common third-party auth tokens
		clonedReq.Header.Del("X-Api-Key")
		clonedReq.Header.Del("Private-Token")
		clonedReq.Header.Del("X-Auth-Token")
	}

	return base.RoundTrip(clonedReq)
}

// NewRedirectHandler creates an http.Client CheckRedirect function that strips credentials
// whenever a redirect navigates away from primaryTargetHost.
func NewRedirectHandler(primaryTargetHost string, strategy Strategy) func(req *http.Request, via []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}

		// Check if redirect is cross-domain
		if !IsSameDomain(req.URL.Host, primaryTargetHost) {
			req.Header.Del("Authorization")
			req.Header.Del("Proxy-Authorization")
			req.Header.Del("Cookie")

			if strategy != nil {
				for _, h := range strategy.Headers() {
					req.Header.Del(h)
				}
			}

			req.Header.Del("X-Api-Key")
			req.Header.Del("Private-Token")
			req.Header.Del("X-Auth-Token")
		}
		return nil
	}
}

// NewScopedHTTPClient constructs a configured *http.Client with domain-scoped transport and redirect handling.
func NewScopedHTTPClient(targetHost string, strategy Strategy, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &ScopedTransport{
			Base:       http.DefaultTransport,
			TargetHost: targetHost,
			Strategy:   strategy,
		},
		CheckRedirect: NewRedirectHandler(targetHost, strategy),
	}
}
