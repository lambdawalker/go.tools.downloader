# Package `model` (`internal/model`)

`internal/model` houses core data structures, status enumerations, session entities, and byte formatting helpers.

## Purpose

The package defines standard schemas shared across the engine, queue, watchdog, store, and UI layers.

## Key Types & Functions

- [`JobStatus`](model.go#L10): Lifecycle states of a job (`StatusPending`, `StatusProbing`, `StatusInProgress`, `StatusPaused`, `StatusDemoted`, `StatusCompleted`, `StatusFailed`).
- [`StreamState`](model.go#L29): Real-time network stream state (`StateConnecting`, `StateWarmingUp`, `StateStable`, `StateStalled`, `StateIdle`).
- [`FileJob`](model.go#L43): Represents an individual file download task, its URLs, files, HTTP headers (ETag, Last-Modified, Accept-Ranges), checksums, and runtime statistics.
- [`Session`](model.go#L92): Encapsulates a batch session containing multiple jobs and session metadata.
- [`FormatBytes(bytes int64) string`](model.go#L101): Formats byte totals into human-readable binary IEC units (e.g., `12.5 MiB`, `1.2 GiB`).
- [`FormatSpeed(bytesPerSec float64) string`](model.go#L118): Formats transfer rates into human-readable speeds (e.g., `3.50 MB/s`).
