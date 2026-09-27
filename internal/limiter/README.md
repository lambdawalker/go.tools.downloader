# Package `limiter` (`internal/limiter`)

`internal/limiter` provides bandwidth throttling and rate-limiting primitives based on the token-bucket algorithm.

## Purpose

The package is responsible for:
- Enforcing global download speed limits across all concurrent workers.
- Regulating throughput smoothly using sub-millisecond sleep intervals and token refills.
- Wrapping standard `io.Reader` interfaces seamlessly via `ThrottledReader` with cancellation support via `context.Context`.

## Key Types & Functions

- [`RateLimiter`](file:///D:/dev/downloader/internal/limiter/limiter.go#L11): Thread-safe token-bucket limiter managing token capacity, refills, and wait queues.
- [`NewRateLimiter(bytesPerSec int64) *RateLimiter`](file:///D:/dev/downloader/internal/limiter/limiter.go#L21): Constructs a new rate limiter with a minimum burst capacity of 64 KB.
- [`Wait(ctx context.Context, bytesCount int) error`](file:///D:/dev/downloader/internal/limiter/limiter.go#L37): Blocks until tokens are available for the requested byte count or context is canceled.
- [`ThrottledReader`](file:///D:/dev/downloader/internal/limiter/limiter.go#L89): Wraps an `io.Reader` to apply token-bucket rate limiting transparently on each `Read` call.
- [`NewThrottledReader(r io.Reader, rl *RateLimiter, ctx context.Context) io.Reader`](file:///D:/dev/downloader/internal/limiter/limiter.go#L97): Constructor for creating a throttled reader wrapper.
