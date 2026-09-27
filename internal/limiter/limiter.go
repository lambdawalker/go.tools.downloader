// Package limiter provides token-bucket bandwidth throttling primitives for streaming readers.
package limiter

import (
	"context"
	"io"
	"sync"
	"time"
)

// RateLimiter enforces a maximum throughput rate measured in bytes per second.
type RateLimiter struct {
	mu           sync.Mutex
	bytesPerSec  float64
	capacity     float64
	tokens       float64
	lastRefilled time.Time
}

// NewRateLimiter creates a new RateLimiter allowing up to bytesPerSec.
// If bytesPerSec is <= 0, nil is returned indicating no rate limiting.
func NewRateLimiter(bytesPerSec int64) *RateLimiter {
	if bytesPerSec <= 0 {
		return nil
	}
	cap := float64(bytesPerSec)
	if cap < 64*1024 {
		cap = 64 * 1024
	}
	return &RateLimiter{
		bytesPerSec:  float64(bytesPerSec),
		capacity:     cap,
		tokens:       cap,
		lastRefilled: time.Now(),
	}
}

// Wait blocks until the required token allowance for bytesCount has been acquired or ctx is canceled.
func (rl *RateLimiter) Wait(ctx context.Context, bytesCount int) error {
	if rl == nil || bytesCount <= 0 {
		return nil
	}

	for bytesCount > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		rl.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(rl.lastRefilled).Seconds()
		rl.tokens += elapsed * rl.bytesPerSec
		if rl.tokens > rl.capacity {
			rl.tokens = rl.capacity
		}
		rl.lastRefilled = now

		take := float64(bytesCount)
		if take > rl.tokens {
			take = rl.tokens
		}

		if take >= 1.0 {
			rl.tokens -= take
			bytesCount -= int(take)
			rl.mu.Unlock()
			continue
		}

		needed := 1.0 - rl.tokens
		sleepSecs := needed / rl.bytesPerSec
		rl.mu.Unlock()

		sleepDuration := time.Duration(sleepSecs * float64(time.Second))
		if sleepDuration < 2*time.Millisecond {
			sleepDuration = 2 * time.Millisecond
		}

		timer := time.NewTimer(sleepDuration)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

// WaitN satisfies the grab.RateLimiter interface by delegating to Wait.
func (rl *RateLimiter) WaitN(ctx context.Context, n int) error {
	return rl.Wait(ctx, n)
}

// ThrottledReader wraps an io.Reader to limit transfer rate using a RateLimiter.
type ThrottledReader struct {
	r   io.Reader
	rl  *RateLimiter
	ctx context.Context
}

// NewThrottledReader wraps r with rate limiting enforced by rl.
// If rl is nil, the original reader r is returned directly without wrapping.
func NewThrottledReader(r io.Reader, rl *RateLimiter, ctx context.Context) io.Reader {
	if rl == nil {
		return r
	}
	return &ThrottledReader{
		r:   r,
		rl:  rl,
		ctx: ctx,
	}
}

// Read reads up to len(p) bytes from the underlying reader and waits for the corresponding rate-limiting allowance.
func (tr *ThrottledReader) Read(p []byte) (int, error) {
	n, err := tr.r.Read(p)
	if n > 0 && tr.rl != nil {
		if waitErr := tr.rl.Wait(tr.ctx, n); waitErr != nil {
			return n, waitErr
		}
	}
	return n, err
}
