// Package transfer implements HTTP file streaming, range-based resumption,
// sliding-window throughput tracking, and atomic final file placement.
package transfer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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

// Downloader executes resumable HTTP downloads with optional rate limiting.
type Downloader struct {
	client  *http.Client
	limiter *limiter.RateLimiter
}

// NewDownloader creates a new Downloader configured with the provided rate limiter (or nil if unlimited).
func NewDownloader(rl *limiter.RateLimiter) *Downloader {
	return &Downloader{
		client: &http.Client{
			Timeout: 0, // No blanket timeout; controlled by context & watchdog
		},
		limiter: rl,
	}
}

// Download downloads the file specified in job into outputDir, resuming from existing .part files if supported.
func (d *Downloader) Download(ctx context.Context, job *model.FileJob, outputDir string) error {
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

	// 2. Inspect existing .part file size
	var existingBytes int64 = 0
	if info, err := os.Stat(partFile); err == nil {
		existingBytes = info.Size()
	}

	// If .part is already full size, verify and promote
	if job.TotalSize > 0 && existingBytes == job.TotalSize {
		if err := os.Rename(partFile, targetFile); err == nil {
			if ok, hash, _ := checksum.VerifyFile(targetFile, job.ExpectedSHA256); ok {
				job.ActualSHA256 = hash
				job.DownloadedBytes = existingBytes
				job.Status = model.StatusCompleted
				return nil
			}
		}
	}

	// 3. Construct HTTP request with Range & If-Range headers
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, job.URL, nil)
	if err != nil {
		return fmt.Errorf("failed creating request: %w", err)
	}
	req.Header.Set("User-Agent", "Go-Concurrent-Downloader/1.0")

	if existingBytes > 0 && job.AcceptRanges {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingBytes))
		// Only attach strong ETags to If-Range (RFC 7232: weak ETags forbidden in range requests)
		if job.ETag != "" && !strings.HasPrefix(job.ETag, "W/") {
			req.Header.Set("If-Range", job.ETag)
		} else if job.LastModified != "" {
			req.Header.Set("If-Range", job.LastModified)
		}
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 4. Handle HTTP response codes
	var outFile *os.File
	var startOffset int64 = 0

	switch resp.StatusCode {
	case http.StatusPartialContent: // 206
		outFile, err = os.OpenFile(partFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed opening part file: %w", err)
		}
		startOffset = existingBytes
	case http.StatusOK: // 200
		// Server ignored range or new download; truncate part file
		outFile, err = os.OpenFile(partFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("failed creating part file: %w", err)
		}
		startOffset = 0
		if resp.ContentLength > 0 {
			job.TotalSize = resp.ContentLength
		}
	case http.StatusRequestedRangeNotSatisfiable: // 416
		// Might already be complete or file changed on server
		if existingBytes > 0 {
			_ = os.Remove(partFile)
		}
		return fmt.Errorf("range not satisfiable (416)")
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return fmt.Errorf("server throttled (HTTP %d)", resp.StatusCode)
	default:
		return fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}
	defer outFile.Close()

	// 5. Stream data with rate limiting, speed tracking, and progress reporting
	speedTracker := NewSpeedTracker(30 * time.Second)
	throttledBody := limiter.NewThrottledReader(resp.Body, d.limiter, ctx)

	buf := make([]byte, 64*1024)
	downloaded := startOffset
	job.SetProgress(downloaded, 0)

	lastProgressFlush := time.Now()

	for {
		nr, rerr := throttledBody.Read(buf)
		if nr > 0 {
			nw, werr := outFile.Write(buf[:nr])
			if werr != nil {
				return fmt.Errorf("write error: %w", werr)
			}
			if nw > 0 {
				downloaded += int64(nw)
			}

			// Update speed & progress periodically
			if time.Since(lastProgressFlush) >= 100*time.Millisecond {
				curSpeed := speedTracker.Update(downloaded)
				job.SetProgress(downloaded, curSpeed)
				lastProgressFlush = time.Now()
			}
		}

		if rerr != nil {
			// Flush file buffer before checking EOF or error
			_ = outFile.Sync()
			if rerr == io.EOF {
				break
			}
			return rerr
		}
	}

	_ = outFile.Sync()
	_ = outFile.Close()

	// 6. Complete and promote
	job.DownloadedBytes = downloaded
	if job.TotalSize > 0 && downloaded < job.TotalSize {
		return fmt.Errorf("incomplete download: expected %d bytes, got %d", job.TotalSize, downloaded)
	}

	// SHA-256 Checksum check if specified
	if job.ExpectedSHA256 != "" {
		match, actual, err := checksum.VerifyFile(partFile, job.ExpectedSHA256)
		if err != nil {
			return fmt.Errorf("checksum calculation error: %w", err)
		}
		job.ActualSHA256 = actual
		if !match {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", job.ExpectedSHA256, actual)
		}
	}

	// Atomic promotion from .part to final destination
	if err := os.Rename(partFile, targetFile); err != nil {
		return fmt.Errorf("failed promoting .part file: %w", err)
	}

	job.Status = model.StatusCompleted
	return nil
}
