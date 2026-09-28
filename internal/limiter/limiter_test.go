package limiter

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	// Zero/negative limiter returns nil
	if NewRateLimiter(0) != nil {
		t.Fatalf("expected nil for 0 bps")
	}

	rl := NewRateLimiter(1024 * 1024) // 1 MB/s
	ctx := context.Background()

	// Wait small amount
	err := rl.Wait(ctx, 1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// WaitN satisfies grab.RateLimiter
	err = rl.WaitN(ctx, 1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Context cancellation
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	err = rl.Wait(cancelCtx, 100*1024*1024)
	if err == nil {
		t.Fatalf("expected error on canceled context")
	}
}

func TestThrottledReader(t *testing.T) {
	data := []byte("hello rate-limited world")
	buf := bytes.NewReader(data)

	// nil limiter returns original reader
	orig := NewThrottledReader(buf, nil, context.Background())
	if orig != buf {
		t.Fatalf("expected original reader when limiter is nil")
	}

	// wrapped reader
	rl := NewRateLimiter(1024 * 1024)
	throttled := NewThrottledReader(bytes.NewReader(data), rl, context.Background())
	readBuf := make([]byte, len(data))
	n, err := io.ReadFull(throttled, readBuf)
	if err != nil && err != io.EOF {
		t.Fatalf("failed reading: %v", err)
	}
	if n != len(data) || !bytes.Equal(readBuf, data) {
		t.Fatalf("data mismatch")
	}

	// Canceled context throttled reader
	cancelCtx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)
	throttledCanceled := NewThrottledReader(bytes.NewReader(data), rl, cancelCtx)
	_, _ = throttledCanceled.Read(make([]byte, 10))
}
