# Package `transfer` (`internal/transfer`)

`internal/transfer` manages HTTP streaming, partial-content resumption, real-time speed tracking, and safe file promotion.

## Purpose

The package is responsible for:
- Detecting previously partially downloaded `.part` files and constructing HTTP `Range: bytes=<offset>-` requests.
- Validating cached data using conditional `If-Range` headers (supporting strong ETags and Last-Modified timestamps per RFC 7232).
- Tracking network speeds using a moving sliding window (`SpeedTracker`).
- Streaming data through token-bucket rate limiters into disk buffers.
- Verifying post-download SHA-256 hashes when specified.
- Atomically promoting completed `.part` files to their final filenames.

## Key Types & Methods

- [`SpeedTracker`](file:///D:/dev/downloader/internal/transfer/transfer.go#L16): Calculates smoothed transfer speeds over a sliding sample window (default 30 seconds).
- [`Downloader`](file:///D:/dev/downloader/internal/transfer/transfer.go#L53): Executes HTTP downloads and manages disk writes.
- [`NewDownloader(rl *limiter.RateLimiter) *Downloader`](file:///D:/dev/downloader/internal/transfer/transfer.go#L59): Constructs a Downloader with rate limiting support.
- [`Download(ctx context.Context, job *model.FileJob, outputDir string) error`](file:///D:/dev/downloader/internal/transfer/transfer.go#L69): Streams the file to a `.part` file, resuming if possible, verifying checksums, and renaming upon success.
