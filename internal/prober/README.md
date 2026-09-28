# Package `prober` (`internal/prober`)

`internal/prober` inspects remote URLs before initiating downloads to extract metadata, detect resume capabilities, and determine target filenames.

## Purpose

The package is responsible for:
- Querying remote HTTP/HTTPS servers via `HEAD` requests (falling back to `Range: bytes=0-0` GET requests if `HEAD` fails).
- Parsing `Content-Length`, `ETag`, and `Last-Modified` headers.
- Verifying partial download capabilities via `Accept-Ranges` or `Content-Range`.
- Resolving canonical filenames from `Content-Disposition` (supporting RFC 5987 / 6266 encoding) or URL paths.

## Key Types & Methods

- [`ProbeResult`](prober.go#L16): Contains file metadata extracted from the remote server.
- [`Prober`](prober.go#L26): Manages probing requests using a configured HTTP client.
- [`NewProber(timeout time.Duration) *Prober`](prober.go#L31): Constructs a new prober with automatic redirect handling and configurable timeouts.
- [`ProbeURL(ctx context.Context, targetURL string) (*ProbeResult, error)`](prober.go#L48): Queries target URL and extracts metadata.
- [`PopulateJob(ctx context.Context, job *model.FileJob) error`](prober.go#L182): Enriches a `FileJob` with probe results.
- [`ProbeURLWithAuth(ctx context.Context, targetURL string, strat auth.Strategy) (*ProbeResult, error)`](prober.go#L187): Queries authenticated endpoints safely using domain scoping.
