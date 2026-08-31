package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/go-telegram/bot"
)

// retryConfig groups the retry knobs so they're easy to tune without
// touching call sites.
type retryConfig struct {
	MaxAttempts  int
	BaseDelay    time.Duration
	MaxDelay     time.Duration
	CheckTimeout time.Duration // passed to bot.WithCheckInitTimeout
}

func defaultRetryConfig() retryConfig {
	return retryConfig{
		MaxAttempts: 5,
		BaseDelay:   1 * time.Second,
		MaxDelay:    15 * time.Second,
		// Slightly more generous than the library's own 5s default, to
		// tolerate connections that are just slow rather than actually
		// blocked -- the retry loop still catches true failures, this
		// just avoids killing borderline-slow ones prematurely.
		CheckTimeout: 10 * time.Second,
	}
}

// retryWithBackoff calls fn up to maxAttempts times, waiting an
// exponentially increasing delay (capped at maxDelay) between attempts.
// Each delay is jittered by ±25% to avoid synchronized retries across
// multiple instances. It returns nil as soon as fn succeeds, or an error
// wrapping the last failure once attempts are exhausted. It also stops
// early if ctx is cancelled while waiting between attempts.
func retryWithBackoff(ctx context.Context, maxAttempts int, baseDelay, maxDelay time.Duration, fn func(attempt int) error) error {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := fn(attempt); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if attempt == maxAttempts {
			break
		}

		delay := baseDelay * time.Duration(1<<(attempt-1))
		if delay > maxDelay {
			delay = maxDelay
		}
		delay = jitterDuration(delay)

		select {
		case <-ctx.Done():
			return fmt.Errorf("cancelled after %d attempt(s): %w", attempt, ctx.Err())
		case <-time.After(delay):
		}
	}

	return fmt.Errorf("failed after %d attempt(s): %w", maxAttempts, lastErr)
}

// jitterDuration returns d scaled by a random factor in [0.75, 1.25].
func jitterDuration(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	// rand.Float64 is in [0,1); map to [0.75, 1.25).
	factor := 0.75 + rand.Float64()*0.5
	return time.Duration(float64(d) * factor)
}

// newBotWithRetry wraps bot.New (which performs an internal getMe() check
// against Telegram) with bounded retry + backoff.
//
// Why this is needed: bot.New's internal getMe check has its own short
// timeout (5s by default, tunable via bot.WithCheckInitTimeout). Sometimes
// the TCP/TLS handshake to api.telegram.org succeeds -- including via the
// IPv6 fast path in telegramTransport -- but the HTTPS request itself
// stalls: e.g. a transient block that lets the handshake through but drops
// the data, surfacing as "context deadline exceeded" even though
// connectivity is generally fine and often recovers within seconds.
//
// Retrying specifically helps here because each retry triggers a brand
// new dial: net/http does not reuse a connection whose in-flight request
// failed with a context error, so telegramTransport's IPv6/IPv4 race gets
// to run again and may land on a different, working path.
func newBotWithRetry(ctx context.Context, logger *slog.Logger, cfg retryConfig, token string, opts ...bot.Option) (*bot.Bot, error) {
	var result *bot.Bot

	allOpts := append([]bot.Option{bot.WithCheckInitTimeout(cfg.CheckTimeout)}, opts...)

	err := retryWithBackoff(ctx, cfg.MaxAttempts, cfg.BaseDelay, cfg.MaxDelay, func(attempt int) error {
		b, err := bot.New(token, allOpts...)
		if err != nil {
			logger.WarnContext(ctx, "telegram bot init attempt failed",
				slog.Int("attempt", attempt),
				slog.Int("max_attempts", cfg.MaxAttempts),
				slog.Any("error", err),
			)
			return err
		}
		if attempt > 1 {
			logger.InfoContext(ctx, "telegram bot connected after retry",
				slog.Int("attempt", attempt),
			)
		}
		result = b
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
