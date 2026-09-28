# Package `limiter` (`internal/limiter`)

`internal/limiter` provides thread-safe token-bucket bandwidth throttling for network streams.

## Purpose

The package enables users to constrain total download speed across all concurrent worker streams. It meters throughput in bytes per second and wraps standard `io.Reader` streams to enforce granular rate limiting.

## Key Types & Methods

- [`RateLimiter`](limiter.go#L11): Thread-safe token-bucket limiter managing token capacity, refills, and wait queues.
- [`NewRateLimiter(bytesPerSec int64) *RateLimiter`](limiter.go#L21): Constructs a new rate limiter with a minimum burst capacity of 64 KB.
- [`Wait(ctx context.Context, bytesCount int) error`](limiter.go#L37): Blocks until tokens are available for the requested byte count or context is canceled.
- [`WaitN(ctx context.Context, n int) error`](limiter.go#L89): Satisfies the `grab.RateLimiter` interface.
- [`ThrottledReader`](limiter.go#L95): Wraps an `io.Reader` to apply token-bucket rate limiting transparently on each `Read` call.
- [`NewThrottledReader(r io.Reader, rl *RateLimiter, ctx context.Context) io.Reader`](limiter.go#L103): Constructor for creating a throttled reader wrapper.
