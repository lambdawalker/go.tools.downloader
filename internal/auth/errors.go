package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// AuthErrorKind categorizes the specific type of authentication or authorization failure.
type AuthErrorKind string

const (
	// KindUnauthorized indicates invalid, missing, or expired credentials (HTTP 401).
	KindUnauthorized AuthErrorKind = "UNAUTHORIZED"
	// KindForbidden indicates insufficient permissions, forbidden scope, or blocked access (HTTP 403).
	KindForbidden AuthErrorKind = "FORBIDDEN"
	// KindRateLimited indicates request limits or API quotas were exceeded (HTTP 429).
	KindRateLimited AuthErrorKind = "RATE_LIMITED"
	// KindInvalidCredentials indicates missing or malformed local credentials (e.g. empty token).
	KindInvalidCredentials AuthErrorKind = "INVALID_CREDENTIALS"
)

// AuthError is a typed, actionable error detailing an authentication or access failure.
type AuthError struct {
	Kind       AuthErrorKind
	StatusCode int
	Domain     string
	URL        string
	Message    string
	RetryAfter time.Duration
	Err        error
}

// Error formats the AuthError into a clear descriptive message.
func (e *AuthError) Error() string {
	base := fmt.Sprintf("[%s] HTTP %d on domain %q: %s", e.Kind, e.StatusCode, e.Domain, e.Message)
	if e.RetryAfter > 0 {
		base += fmt.Sprintf(" (retry after %s)", e.RetryAfter)
	}
	if e.Err != nil {
		base += fmt.Sprintf(": %v", e.Err)
	}
	return base
}

// Unwrap returns the underlying wrapped error, if any.
func (e *AuthError) Unwrap() error {
	return e.Err
}

// IsAuthError reports whether err or any wrapped error is an *AuthError.
func IsAuthError(err error) bool {
	var authErr *AuthError
	return errors.As(err, &authErr)
}

// AsAuthError attempts to extract an *AuthError from err.
func AsAuthError(err error) (*AuthError, bool) {
	var authErr *AuthError
	if errors.As(err, &authErr) {
		return authErr, true
	}
	return nil, false
}

// IsUnauthorized reports whether err represents an HTTP 401 Unauthorized failure.
func IsUnauthorized(err error) bool {
	if authErr, ok := AsAuthError(err); ok {
		return authErr.Kind == KindUnauthorized || authErr.StatusCode == http.StatusUnauthorized
	}
	return false
}

// IsForbidden reports whether err represents an HTTP 403 Forbidden failure.
func IsForbidden(err error) bool {
	if authErr, ok := AsAuthError(err); ok {
		return authErr.Kind == KindForbidden || authErr.StatusCode == http.StatusForbidden
	}
	return false
}

// IsRateLimited reports whether err represents an HTTP 429 Too Many Requests failure.
func IsRateLimited(err error) bool {
	if authErr, ok := AsAuthError(err); ok {
		return authErr.Kind == KindRateLimited || authErr.StatusCode == http.StatusTooManyRequests
	}
	return false
}

// ParseRetryAfter parses standard HTTP Retry-After headers in seconds or RFC 1123 date format.
func ParseRetryAfter(headerVal string) time.Duration {
	headerVal = headerVal
	if headerVal == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(headerVal); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if t, err := http.ParseTime(headerVal); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

// MapStatusCode converts an HTTP status code and optional headers into a typed AuthError if applicable.
// If the status code is not an authentication/authorization failure, nil is returned.
func MapStatusCode(statusCode int, domain, urlStr string, respHeaders http.Header, origErr error) *AuthError {
	switch statusCode {
	case http.StatusUnauthorized: // 401
		return &AuthError{
			Kind:       KindUnauthorized,
			StatusCode: statusCode,
			Domain:     domain,
			URL:        urlStr,
			Message:    "Authentication failed: credentials missing, invalid, or expired",
			Err:        origErr,
		}
	case http.StatusForbidden: // 403
		return &AuthError{
			Kind:       KindForbidden,
			StatusCode: statusCode,
			Domain:     domain,
			URL:        urlStr,
			Message:    "Access forbidden: insufficient scopes or permissions",
			Err:        origErr,
		}
	case http.StatusTooManyRequests: // 429
		var retryAfter time.Duration
		if respHeaders != nil {
			retryAfter = ParseRetryAfter(respHeaders.Get("Retry-After"))
		}
		return &AuthError{
			Kind:       KindRateLimited,
			StatusCode: statusCode,
			Domain:     domain,
			URL:        urlStr,
			Message:    "Rate limited: too many requests received by server",
			RetryAfter: retryAfter,
			Err:        origErr,
		}
	default:
		return nil
	}
}
