// Package listenbrainz implements core.Player and core.ArtistGetter using
// ListenBrainz (now-playing / recent listens) plus MusicBrainz and Wikidata
// for artist biographies.
package listenbrainz

import (
	"context"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func isRetryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return code >= 500
	}
}

func backoff(p retryPolicy, attempt int) time.Duration {
	d := min(p.baseDelay*time.Duration(1<<attempt), p.maxDelay)
	half := int64(d / 2)
	jitter := time.Duration(rand.Int63n(half + 1))
	return d/2 + jitter
}

func retryAfter(h http.Header, p retryPolicy, attempt int) time.Duration {
	v := h.Get("Retry-After")
	if v == "" {
		return backoff(p, attempt)
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return backoff(p, attempt)
}
