// Package prober inspects remote HTTP endpoints to pre-determine file metadata,
// size, range support, and canonical filenames prior to initiating downloads.
package prober

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"downloader/internal/auth"
	"downloader/internal/model"
)

// ProbeResult encapsulates metadata discovered during HTTP URL probing.
type ProbeResult struct {
	TotalSize        int64
	AcceptRanges     bool
	ETag             string
	LastModified     string
	ResolvedFilename string
	RetryAfter       time.Duration
}

// Prober inspects target URLs using HEAD and Range GET requests.
type Prober struct {
	client  *http.Client
	timeout time.Duration
}

// NewProber initializes a Prober configured with the specified HTTP request timeout.
func NewProber(timeout time.Duration) *Prober {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Prober{
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				return nil
			},
		},
	}
}

// ProbeURL queries targetURL using HEAD (or Range GET fallback) to determine file size, range support, and filename.
func (p *Prober) ProbeURL(ctx context.Context, targetURL string) (*ProbeResult, error) {
	return p.ProbeURLWithAuth(ctx, targetURL, nil)
}

// ProbeURLWithAuth queries targetURL using HEAD (or Range GET fallback) applying domain-scoped auth credentials.
func (p *Prober) ProbeURLWithAuth(ctx context.Context, targetURL string, authStrategy auth.Strategy) (*ProbeResult, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	result := &ProbeResult{
		TotalSize: -1,
	}

	scopedClient := auth.NewScopedHTTPClient(parsedURL.Host, authStrategy, p.timeout)

	// 1. Try HEAD request first
	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	if err == nil {
		headReq.Header.Set("User-Agent", "Go-Concurrent-Downloader/1.0")
		resp, headErr := scopedClient.Do(headReq)
		if headErr == nil {
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				p.extractMetadata(resp, result, parsedURL)
				if result.TotalSize > 0 && result.ResolvedFilename != "" {
					return result, nil
				}
			} else if authErr := auth.MapStatusCode(resp.StatusCode, parsedURL.Hostname(), targetURL, resp.Header, nil); authErr != nil {
				return result, authErr
			}
		}
	}

	// 2. Fallback to GET with Range: bytes=0-0
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed creating get request: %w", err)
	}
	getReq.Header.Set("User-Agent", "Go-Concurrent-Downloader/1.0")
	getReq.Header.Set("Range", "bytes=0-0")

	resp, err := scopedClient.Do(getReq)
	if err != nil {
		result.ResolvedFilename = sanitizeFilename(extractFilenameFromURL(parsedURL), "")
		return result, fmt.Errorf("probe request failed: %w", err)
	}
	defer resp.Body.Close()

	if authErr := auth.MapStatusCode(resp.StatusCode, parsedURL.Hostname(), targetURL, resp.Header, nil); authErr != nil {
		return result, authErr
	}

	if resp.StatusCode == http.StatusPartialContent { // 206
		result.AcceptRanges = true
		contentRange := resp.Header.Get("Content-Range")
		if contentRange != "" {
			parts := strings.Split(contentRange, "/")
			if len(parts) == 2 && parts[1] != "*" {
				if size, parseErr := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); parseErr == nil {
					result.TotalSize = size
				}
			}
		}
	} else if resp.StatusCode == http.StatusOK { // 200
		if resp.ContentLength > 0 {
			result.TotalSize = resp.ContentLength
		}
	} else if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		result.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
		return result, fmt.Errorf("server returned status: %d", resp.StatusCode)
	}

	p.extractMetadata(resp, result, parsedURL)
	return result, nil
}

// extractMetadata populates size, headers, and filename candidates from an HTTP response.
func (p *Prober) extractMetadata(resp *http.Response, result *ProbeResult, parsedURL *url.URL) {
	if resp.ContentLength > 0 && result.TotalSize <= 0 {
		result.TotalSize = resp.ContentLength
	}

	if strings.EqualFold(resp.Header.Get("Accept-Ranges"), "bytes") {
		result.AcceptRanges = true
	}

	// Capture ETag
	etag := resp.Header.Get("ETag")
	if etag != "" {
		result.ETag = etag
	}

	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		result.LastModified = lm
	}

	// Filename resolution:
	// 1. Content-Disposition
	var candidateName string
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" {
		candidateName = parseContentDisposition(cd)
	}

	// 2. Fallback to URL path
	if candidateName == "" {
		candidateName = extractFilenameFromURL(parsedURL)
	}

	// 3. Fallback to Content-Type extension if needed
	ct := resp.Header.Get("Content-Type")
	result.ResolvedFilename = sanitizeFilename(candidateName, ct)
}

// parseContentDisposition extracts a filename from a Content-Disposition header, favoring RFC 5987 filename*.
func parseContentDisposition(header string) string {
	_, params, err := mime.ParseMediaType(header)
	if err == nil {
		// RFC 5987 / RFC 6266 filename* takes precedence
		if fnExt, ok := params["filename*"]; ok && fnExt != "" {
			// format: UTF-8''encoded%20string
			parts := strings.SplitN(fnExt, "''", 2)
			if len(parts) == 2 {
				if decoded, unescapeErr := url.PathUnescape(parts[1]); unescapeErr == nil && decoded != "" {
					return decoded
				}
			}
		}
		if fn, ok := params["filename"]; ok && fn != "" {
			return fn
		}
	}
	return ""
}

// extractFilenameFromURL extracts a filename from the URL path, defaulting to "download.bin".
func extractFilenameFromURL(u *url.URL) string {
	cleanPath := strings.TrimRight(u.Path, "/")
	base := filepath.Base(cleanPath)
	if base == "." || base == "/" || base == "" {
		return "download.bin"
	}
	return base
}

// sanitizeFilename strips path traversal separators and unprintable characters, resolving extensions if missing.
func sanitizeFilename(name, contentType string) string {
	name = filepath.Base(filepath.Clean(name))
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "\x00", "")

	// Remove unprintable characters
	var sb strings.Builder
	for _, r := range name {
		if r >= 32 && r != 127 {
			sb.WriteRune(r)
		}
	}
	sanitized := strings.TrimSpace(sb.String())
	if sanitized == "" || sanitized == "." {
		sanitized = "download.bin"
	}

	// Check if extension exists, otherwise deduce from content type
	ext := filepath.Ext(sanitized)
	if ext == "" && contentType != "" {
		mediaType, _, _ := mime.ParseMediaType(contentType)
		exts, _ := mime.ExtensionsByType(mediaType)
		if len(exts) > 0 {
			sanitized += exts[0]
		}
	}
	return sanitized
}

// parseRetryAfter parses an HTTP Retry-After header as either seconds or an HTTP-formatted date.
func parseRetryAfter(val string) time.Duration {
	if val == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(val); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if t, err := http.ParseTime(val); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

// PopulateJob executes ProbeURL for the given job and enriches its fields with the discovered metadata.
func (p *Prober) PopulateJob(ctx context.Context, job *model.FileJob) error {
	return p.PopulateJobWithAuth(ctx, job, nil)
}

// PopulateJobWithAuth executes ProbeURLWithAuth for the job and enriches its fields with discovered metadata.
func (p *Prober) PopulateJobWithAuth(ctx context.Context, job *model.FileJob, authStrategy auth.Strategy) error {
	res, err := p.ProbeURLWithAuth(ctx, job.URL, authStrategy)
	if res != nil {
		if res.TotalSize > 0 {
			job.TotalSize = res.TotalSize
		}
		job.AcceptRanges = res.AcceptRanges
		job.ETag = res.ETag
		job.LastModified = res.LastModified
		if job.ResolvedFilename == "" {
			job.ResolvedFilename = res.ResolvedFilename
		}
	}
	return err
}
