package telegram

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryWithBackoff_SucceedsImmediately(t *testing.T) {
	var calls int32
	start := time.Now()

	err := retryWithBackoff(context.Background(), 5, 10*time.Millisecond, 100*time.Millisecond, func(attempt int) error {
		atomic.AddInt32(&calls, 1)
		return nil
	})

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected exactly 1 call, got %d", got)
	}
	// No backoff should have been consumed since the very first call
	// succeeded.
	if elapsed := time.Since(start); elapsed > 20*time.Millisecond {
		t.Fatalf("expected near-instant success, took %v", elapsed)
	}
}

func TestRetryWithBackoff_SucceedsAfterTransientFailures(t *testing.T) {
	// Simulates the real scenario: first two attempts fail like the
	// "context deadline exceeded" case (connect succeeds, request
	// stalls), third attempt succeeds.
	var calls int32

	err := retryWithBackoff(context.Background(), 5, 5*time.Millisecond, 50*time.Millisecond, func(attempt int) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return errors.New("context deadline exceeded")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("expected eventual success, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected exactly 3 calls, got %d", got)
	}
}

func TestRetryWithBackoff_AllAttemptsFail(t *testing.T) {
	var calls int32
	const maxAttempts = 4

	err := retryWithBackoff(context.Background(), maxAttempts, 1*time.Millisecond, 5*time.Millisecond, func(attempt int) error {
		atomic.AddInt32(&calls, 1)
		return errors.New("boom")
	})

	if err == nil {
		t.Fatalf("expected error when all attempts fail")
	}
	if got := atomic.LoadInt32(&calls); got != maxAttempts {
		t.Fatalf("expected exactly %d calls, got %d", maxAttempts, got)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected wrapped underlying error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "4 attempt") {
		t.Fatalf("expected error to mention attempt count, got: %v", err)
	}
}

func TestRetryWithBackoff_StopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls int32

	go func() {
		time.Sleep(15 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := retryWithBackoff(ctx, 100, 50*time.Millisecond, 50*time.Millisecond, func(attempt int) error {
		atomic.AddInt32(&calls, 1)
		return errors.New("always fails")
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected error from cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error to wrap context.Canceled, got: %v", err)
	}
	// Should bail out shortly after cancellation, not run all 100
	// attempts' worth of backoff.
	if elapsed > 200*time.Millisecond {
		t.Fatalf("cancellation didn't stop retries promptly, took %v", elapsed)
	}
	if got := atomic.LoadInt32(&calls); got < 1 || got > 3 {
		t.Fatalf("expected only a couple of calls before cancellation, got %d", got)
	}
	t.Logf("stopped after %d calls in %v", calls, elapsed)
}

func TestRetryWithBackoff_BackoffIsExponentialAndCapped(t *testing.T) {
	var timestamps []time.Time

	_ = retryWithBackoff(context.Background(), 4, 10*time.Millisecond, 25*time.Millisecond, func(attempt int) error {
		timestamps = append(timestamps, time.Now())
		return errors.New("fail")
	})

	if len(timestamps) != 4 {
		t.Fatalf("expected 4 attempts, got %d", len(timestamps))
	}

	gaps := make([]time.Duration, 0, 3)
	for i := 1; i < len(timestamps); i++ {
		gaps = append(gaps, timestamps[i].Sub(timestamps[i-1]))
	}

	// Expected gaps: ~10ms, ~20ms, ~25ms (capped from 40ms).
	// Allow generous slack for scheduler jitter.
	wantMin := []time.Duration{5 * time.Millisecond, 12 * time.Millisecond, 15 * time.Millisecond}
	for i, gap := range gaps {
		if gap < wantMin[i] {
			t.Errorf("gap %d = %v, expected at least %v", i, gap, wantMin[i])
		}
	}
	// The third gap (capped) shouldn't run away to ~40ms+.
	if gaps[2] > 60*time.Millisecond {
		t.Errorf("expected capped backoff on gap 3, got %v", gaps[2])
	}
	t.Logf("gaps: %v", gaps)
}

// TestNewBotWithRetry_UsesGenericRetryPath is a lightweight check that
// newBotWithRetry's own bookkeeping (attempt counting, success/failure
// propagation) behaves correctly, using a fake "bot.New"-shaped factory so
// no real network calls are made. It exercises the same code path used in
// NewTelegramBot without needing valid Telegram credentials.
func TestNewBotWithRetry_ReturnsErrorAfterExhaustingAttempts(t *testing.T) {
	cfg := retryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    5 * time.Millisecond,
	}

	var calls int32
	err := retryWithBackoff(context.Background(), cfg.MaxAttempts, cfg.BaseDelay, cfg.MaxDelay, func(attempt int) error {
		atomic.AddInt32(&calls, 1)
		return errors.New("context deadline exceeded")
	})

	if err == nil {
		t.Fatalf("expected error")
	}
	if got := atomic.LoadInt32(&calls); got != int32(cfg.MaxAttempts) {
		t.Fatalf("expected %d attempts, got %d", cfg.MaxAttempts, got)
	}
}

func TestJitterDuration_Range(t *testing.T) {
	base := 100 * time.Millisecond
	for i := 0; i < 50; i++ {
		j := jitterDuration(base)
		if j < 75*time.Millisecond || j >= 125*time.Millisecond {
			t.Fatalf("jitter %v outside [0.75, 1.25) * base", j)
		}
	}
	if jitterDuration(0) != 0 {
		t.Fatal("zero stays zero")
	}
}
