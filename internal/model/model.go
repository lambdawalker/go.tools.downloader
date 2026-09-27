// Package model defines core domain entities, status enums, and data structures
// representing download jobs, streams, and persistent sessions.
package model

import (
	"fmt"
	"sync"
	"time"
)

// JobStatus describes the overall processing state of a FileJob.
type JobStatus string

const (
	// StatusPending indicates the job is queued and waiting to be processed.
	StatusPending JobStatus = "PENDING"
	// StatusProbing indicates the job URL is currently being inspected for headers and range support.
	StatusProbing JobStatus = "PROBING"
	// StatusInProgress indicates the job is currently being downloaded by an active worker.
	StatusInProgress JobStatus = "DOWNLOADING"
	// StatusPaused indicates the job was interrupted or canceled and can be resumed.
	StatusPaused JobStatus = "PAUSED"
	// StatusDemoted indicates the job was demoted to the back of the queue due to slow transfer or stalls.
	StatusDemoted JobStatus = "DEMOTED"
	// StatusCompleted indicates the file was fully downloaded and verified successfully.
	StatusCompleted JobStatus = "COMPLETED"
	// StatusFailed indicates the download or verification failed after exhausting retry attempts.
	StatusFailed JobStatus = "FAILED"
)

// StreamState represents the real-time operational status of an active HTTP transfer stream.
type StreamState string

const (
	// StateConnecting indicates the HTTP connection or initial request is in progress.
	StateConnecting StreamState = "CONNECTING"
	// StateWarmingUp indicates data is being received but the stream is still within its initial warmup window.
	StateWarmingUp StreamState = "WARMING_UP"
	// StateStable indicates the stream is actively transferring data with stable throughput.
	StateStable StreamState = "STABLE"
	// StateStalled indicates no bytes have been received within the stall timeout window.
	StateStalled StreamState = "STALLED"
	// StateIdle indicates the worker is idle without an active stream.
	StateIdle StreamState = "IDLE"
)

// FileJob encapsulates all metadata, progress counters, checksums, and runtime state for a single download.
type FileJob struct {
	ID               string      `json:"id"`
	URL              string      `json:"url"`
	ResolvedFilename string      `json:"resolved_filename"`
	TargetFile       string      `json:"target_file"`
	PartFile         string      `json:"part_file"`
	TotalSize        int64       `json:"total_size"` // -1 if unknown
	DownloadedBytes  int64       `json:"downloaded_bytes"`
	ETag             string      `json:"etag"`
	LastModified     string      `json:"last_modified"`
	AcceptRanges     bool        `json:"accept_ranges"`
	ExpectedSHA256   string      `json:"expected_sha256,omitempty"`
	ActualSHA256     string      `json:"actual_sha256,omitempty"`
	Status           JobStatus   `json:"status"`
	StreamState      StreamState `json:"stream_state"`
	DemotedOnce      bool        `json:"demoted_once"`
	RetryCount       int         `json:"retry_count"`
	LastError        string      `json:"last_error,omitempty"`

	// Runtime metrics (not persisted)
	mu                sync.RWMutex
	CurrentSpeed      float64   `json:"-"` // bytes per second
	StateEnteredAt    time.Time `json:"-"`
	LastByteReceived  time.Time `json:"-"`
	DownloadStartedAt time.Time `json:"-"`
}

// SetProgress thread-safely updates the downloaded byte count, current transfer speed, and last activity timestamp.
func (j *FileJob) SetProgress(downloaded int64, speed float64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.DownloadedBytes = downloaded
	j.CurrentSpeed = speed
	j.LastByteReceived = time.Now()
}

// SetStreamState thread-safely transitions the stream state and records the entry timestamp.
func (j *FileJob) SetStreamState(state StreamState) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.StreamState != state {
		j.StreamState = state
		j.StateEnteredAt = time.Now()
	}
}

// GetSnapshot retrieves a point-in-time thread-safe snapshot of download progress, speed, and state.
func (j *FileJob) GetSnapshot() (downloaded int64, total int64, speed float64, state StreamState, status JobStatus) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.DownloadedBytes, j.TotalSize, j.CurrentSpeed, j.StreamState, j.Status
}

// Session represents a batch download session consisting of configuration, timestamps, and constituent jobs.
type Session struct {
	Name      string     `json:"name"`
	OutputDir string     `json:"output_dir"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Jobs      []*FileJob `json:"jobs"`
}

// FormatBytes converts a raw byte count into a human-readable string formatted with binary units (B, KiB, MiB, etc.).
func FormatBytes(bytes int64) string {
	if bytes < 0 {
		return "Unknown"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatSpeed formats a transfer speed in bytes per second into a human-readable transfer rate string.
func FormatSpeed(bytesPerSec float64) string {
	if bytesPerSec <= 0 {
		return "0 B/s"
	}
	const unit = 1024.0
	if bytesPerSec < unit {
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	}
	div, exp := unit, 0
	for n := bytesPerSec / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %c/s", bytesPerSec/div, "KMGTPE"[exp])
}
