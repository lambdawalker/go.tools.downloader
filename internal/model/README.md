# Package `model` (`internal/model`)

`internal/model` defines the shared domain entities, status enums, and formatting helpers used across all downloader packages.

## Purpose

The package is responsible for:
- Modeling download jobs (`FileJob`), persistent sessions (`Session`), and lifecycle states.
- Providing thread-safe progress reporting and snapshot retrieval for active streams.
- Providing formatting utilities to render byte counts and transfer rates into human-readable strings.

## Key Types & Enums

- [`JobStatus`](file:///D:/dev/downloader/internal/model/model.go#L10): Lifecycle states of a job (`StatusPending`, `StatusProbing`, `StatusInProgress`, `StatusPaused`, `StatusDemoted`, `StatusCompleted`, `StatusFailed`).
- [`StreamState`](file:///D:/dev/downloader/internal/model/model.go#L29): Real-time network stream state (`StateConnecting`, `StateWarmingUp`, `StateStable`, `StateStalled`, `StateIdle`).
- [`FileJob`](file:///D:/dev/downloader/internal/model/model.go#L43): Represents an individual file download task, its URLs, files, HTTP headers (ETag, Last-Modified, Accept-Ranges), checksums, and runtime statistics.
- [`Session`](file:///D:/dev/downloader/internal/model/model.go#L91): Encapsulates a batch session containing multiple jobs and session metadata.

## Key Helpers

- [`FormatBytes(bytes int64) string`](file:///D:/dev/downloader/internal/model/model.go#L100): Formats byte totals into human-readable binary IEC units (e.g., `12.5 MiB`, `1.2 GiB`).
- [`FormatSpeed(bytesPerSec float64) string`](file:///D:/dev/downloader/internal/model/model.go#L117): Formats transfer rates into human-readable speeds (e.g., `3.50 MB/s`).
