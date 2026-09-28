# Package `transfer` (`internal/transfer`)

`internal/transfer` manages HTTP streaming, range-based resumption, rate-limited downloads, and checksum validation powered by `github.com/cavaliergopher/grab/v3`, integrated with domain-scoped authentication.

## Purpose

The package is responsible for:
- Integrating `cavaliergopher/grab/v3` as the underlying HTTP streaming and resumption engine.
- Keeping downloads completely site-agnostic: accepting the target URL, destination, and an authentication strategy without hardcoded knowledge of specific websites.
- Enforcing domain-scoped authentication via `auth.ScopedTransport`, guaranteeing credentials are never forwarded across cross-domain redirects (e.g. presigned AWS S3 or Cloudflare CDN URLs).
- Resuming interrupted downloads from partial `.part` files automatically.
- Supporting token-bucket bandwidth throttling via `limiter.RateLimiter`.
- Validating checksums (SHA-256) and atomically promoting `.part` files upon successful completion.
- Mapping HTTP 401, 403, and 429 status codes into actionable, typed `auth.AuthError` instances.

## Key Types & Functions

- [`SpeedTracker`](transfer.go#L23): Calculates smoothed transfer speeds over a sliding sample window.
- [`Downloader`](transfer.go#L76): Coordinates grab file transfers with authentication strategies and rate limiting.
- [`NewDownloader(rl *limiter.RateLimiter) *Downloader`](transfer.go#L81): Constructs a new Downloader.
- [`DownloadURL(ctx context.Context, targetURL, destination string, authStrategy auth.Strategy) (*DownloadResult, error)`](transfer.go#L89): Decoupled standalone transfer function accepting target URL, destination, and authentication strategy.
- [`Download(ctx context.Context, job *model.FileJob, outputDir string, authStrategy auth.Strategy) error`](transfer.go#L140): Downloads a job into outputDir using grab, updating progress in real-time, verifying checksums, and renaming `.part` files.
