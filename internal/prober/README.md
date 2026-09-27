# Package `prober` (`internal/prober`)

`internal/prober` inspects remote HTTP URLs prior to downloading to determine file sizes, resume capabilities, cache validators, and canonical filenames.

## Purpose

The package is responsible for:
- Issuing lightweight HTTP `HEAD` requests to inspect remote headers.
- Falling back to HTTP `GET` with `Range: bytes=0-0` when servers return non-standard responses to `HEAD` or omit length headers.
- Detecting byte-range resumption support (`Accept-Ranges: bytes` or HTTP 206 `Content-Range`).
- Capturing cache headers (`ETag` and `Last-Modified`) for conditional HTTP requests during resumption.
- Resolving canonical target filenames using RFC 5987 / RFC 6266 `Content-Disposition`, URL paths, and MIME type extension inference.
- Sanitizing filenames to eliminate unsafe path traversal characters.

## Key Types & Functions

- [`ProbeResult`](file:///D:/dev/downloader/internal/prober/prober.go#L16): Contains file metadata extracted from the remote server.
- [`Prober`](file:///D:/dev/downloader/internal/prober/prober.go#L26): Manages probing requests using a configured HTTP client.
- [`NewProber(timeout time.Duration) *Prober`](file:///D:/dev/downloader/internal/prober/prober.go#L31): Constructs a new prober with automatic redirect handling and configurable timeouts.
- [`ProbeURL(ctx context.Context, targetURL string) (*ProbeResult, error)`](file:///D:/dev/downloader/internal/prober/prober.go#L48): Queries target URL and extracts metadata.
- [`PopulateJob(ctx context.Context, job *model.FileJob) error`](file:///D:/dev/downloader/internal/prober/prober.go#L182): Enriches a `FileJob` with probe results.
