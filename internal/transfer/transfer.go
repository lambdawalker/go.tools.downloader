// Package transfer implements HTTP file streaming, range-based resumption,
// sliding-window throughput tracking, and grab-powered downloads with authentication support.
package transfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/cavaliergopher/grab/v3"

	"downloader/internal/auth"
	"downloader/internal/checksum"
	"downloader/internal/limiter"
	"downloader/internal/model"
)

// SpeedTracker tracks transfer progress over a sliding time window to calculate current transfer rate.
type SpeedTracker struct {
	window       time.Duration
	samples      []speedSample
	lastBytes    int64
	currentSpeed float64
}

// speedSample stores a byte count observed at a specific point in time.
type speedSample struct {
	timestamp time.Time
	bytes     int64
}

// NewSpeedTracker constructs a new SpeedTracker configured with the specified smoothing window duration.
func NewSpeedTracker(window time.Duration) *SpeedTracker {
	return &SpeedTracker{
		window:  window,
		samples: make([]speedSample, 0, 32),
	}
}

// Update appends currentBytes to the sliding window, purges stale samples, and returns the computed bytes/sec.
func (st *SpeedTracker) Update(currentBytes int64) float64 {
	now := time.Now()
	st.samples = append(st.samples, speedSample{timestamp: now, bytes: currentBytes})

	cutoff := now.Add(-st.window)
	validIdx := 0
	for i, s := range st.samples {
		if s.timestamp.After(cutoff) {
			validIdx = i
			break
		}
	}
	st.samples = st.samples[validIdx:]

	if len(st.samples) >= 2 {
		first := st.samples[0]
		last := st.samples[len(st.samples)-1]
		elapsed := last.timestamp.Sub(first.timestamp).Seconds()
		if elapsed > 0.1 {
			st.currentSpeed = float64(last.bytes-first.bytes) / elapsed
		}
	}
	st.lastBytes = currentBytes
	return st.currentSpeed
}

// DownloadResult summarizes details of a completed transfer.
type DownloadResult struct {
	Filename      string
	BytesComplete int64
	TotalSize     int64
	DidResume     bool
	Duration      time.Duration
	ActualSHA256  string
}

// Downloader executes resumable HTTP downloads via grab with rate limiting and authentication scoping.
type Downloader struct {
	limiter *limiter.RateLimiter
}

// NewDownloader creates a new Downloader configured with the provided rate limiter (or nil if unlimited).
func NewDownloader(rl *limiter.RateLimiter) *Downloader {
	return &Downloader{
		limiter: rl,
	}
}

// DownloadURL downloads the targetURL to destination using grab and the supplied authentication strategy.
// The download is site-agnostic, strictly scoping credentials to targetURL's domain.
func (d *Downloader) DownloadURL(ctx context.Context, targetURL, destination string, authStrategy auth.Strategy) (*DownloadResult, error) {
	if authStrategy == nil {
		authStrategy = auth.NewAnonymousStrategy()
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}

	// 1. Configure domain-scoped HTTP client
	scopedHTTP := auth.NewScopedHTTPClient(parsedURL.Host, authStrategy, 0)

	// 2. Build Grab client
	grabClient := grab.NewClient()
	grabClient.HTTPClient = scopedHTTP
	grabClient.UserAgent = "Go-Concurrent-Downloader/1.0"

	// 3. Create Grab request
	req, err := grab.NewRequest(destination, targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed creating grab request: %w", err)
	}
	req = req.WithContext(ctx)

	if d.limiter != nil {
		req.RateLimiter = d.limiter
	}

	// 4. Start transfer
	resp := grabClient.Do(req)

	select {
	case <-ctx.Done():
		_ = resp.Cancel()
		return nil, ctx.Err()
	case <-resp.Done:
		if err := resp.Err(); err != nil {
			return nil, d.mapError(err, parsedURL.Hostname(), targetURL, resp)
		}
	}

	return &DownloadResult{
		Filename:      resp.Filename,
		BytesComplete: resp.BytesComplete(),
		TotalSize:     resp.Size(),
		DidResume:     resp.DidResume,
		Duration:      resp.Duration(),
	}, nil
}

// Download downloads the file specified in job into outputDir, resuming from existing .part files if supported.
// Authentication credentials from authStrategy are applied strictly to the target domain.
func (d *Downloader) Download(ctx context.Context, job *model.FileJob, outputDir string, authStrategy auth.Strategy) error {
	if authStrategy == nil {
		authStrategy = auth.NewAnonymousStrategy()
	}

	if job.ResolvedFilename == "" {
		job.ResolvedFilename = "download.bin"
	}

	targetFile := filepath.Join(outputDir, job.ResolvedFilename)
	partFile := targetFile + ".part"
	job.TargetFile = targetFile
	job.PartFile = partFile

	// 1. Check if complete file already exists on disk
	if info, err := os.Stat(targetFile); err == nil {
		if job.TotalSize > 0 && info.Size() == job.TotalSize {
			job.DownloadedBytes = info.Size()
			if ok, hash, _ := checksum.VerifyFile(targetFile, job.ExpectedSHA256); ok {
				job.ActualSHA256 = hash
				job.Status = model.StatusCompleted
				return nil
			}
		}
	}

	parsedURL, err := url.Parse(job.URL)
	if err != nil {
		return fmt.Errorf("invalid job URL: %w", err)
	}

	// 2. Configure domain-scoped HTTP client
	scopedHTTP := auth.NewScopedHTTPClient(parsedURL.Host, authStrategy, 0)

	// 3. Build Grab client
	grabClient := grab.NewClient()
	grabClient.HTTPClient = scopedHTTP
	grabClient.UserAgent = "Go-Concurrent-Downloader/1.0"

	// 4. Create Grab request targeting the .part file for resumable atomic placement
	req, err := grab.NewRequest(partFile, job.URL)
	if err != nil {
		return fmt.Errorf("failed creating grab request: %w", err)
	}
	req = req.WithContext(ctx)

	if d.limiter != nil {
		req.RateLimiter = d.limiter
	}

	// Attach checksum if expected
	if job.ExpectedSHA256 != "" {
		if sumBytes, err := hex.DecodeString(job.ExpectedSHA256); err == nil {
			req.SetChecksum(sha256.New(), sumBytes, false)
		}
	}

	// 5. Start transfer
	resp := grabClient.Do(req)

	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = resp.Cancel()
			return ctx.Err()

		case <-ticker.C:
			job.SetProgress(resp.BytesComplete(), resp.BytesPerSecond())

		case <-resp.Done:
			// Final update
			job.SetProgress(resp.BytesComplete(), resp.BytesPerSecond())

			if err := resp.Err(); err != nil {
				return d.mapError(err, parsedURL.Hostname(), job.URL, resp)
			}

			// Atomic promotion from .part to final destination
			if err := os.Rename(partFile, targetFile); err != nil {
				return fmt.Errorf("failed promoting .part file: %w", err)
			}

			// Validate SHA-256 and store actual hash
			if ok, hash, chkErr := checksum.VerifyFile(targetFile, job.ExpectedSHA256); !ok {
				if chkErr != nil {
					return fmt.Errorf("checksum calculation error: %w", chkErr)
				}
				job.ActualSHA256 = hash
				return fmt.Errorf("checksum mismatch: expected %s, got %s", job.ExpectedSHA256, hash)
			} else {
				job.ActualSHA256 = hash
			}

			job.DownloadedBytes = resp.BytesComplete()
			job.Status = model.StatusCompleted
			return nil
		}
	}
}

// mapError translates grab and HTTP status errors into typed auth.AuthError if applicable.
func (d *Downloader) mapError(err error, domain, targetURL string, resp *grab.Response) error {
	if err == nil {
		return nil
	}

	// Check if already an AuthError
	if auth.IsAuthError(err) {
		return err
	}

	// Inspect HTTP status code
	var statusErr grab.StatusCodeError
	if errors.As(err, &statusErr) {
		code := int(statusErr)
		var headers http.Header
		if resp != nil && resp.HTTPResponse != nil {
			headers = resp.HTTPResponse.Header
		}
		if authErr := auth.MapStatusCode(code, domain, targetURL, headers, err); authErr != nil {
			return authErr
		}
	}

	if resp != nil && resp.HTTPResponse != nil {
		code := resp.HTTPResponse.StatusCode
		if authErr := auth.MapStatusCode(code, domain, targetURL, resp.HTTPResponse.Header, err); authErr != nil {
			return authErr
		}
	}

	return err
}
