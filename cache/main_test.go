package cache

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/oklookat/teletrack/shared"
)

func TestSQLiteCache_SetGetDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := newTestCache(t)

	if _, ok := c.Get(ctx, "missing"); ok {
		t.Fatal("expected miss for missing key")
	}

	if err := c.Set(ctx, "k1", []byte("hello"), time.Hour); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, ok := c.Get(ctx, "k1")
	if !ok {
		t.Fatal("expected hit")
	}
	if string(val) != "hello" {
		t.Fatalf("got %q, want hello", val)
	}

	if err := c.Delete(ctx, "k1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := c.Get(ctx, "k1"); ok {
		t.Fatal("expected miss after delete")
	}
}

func TestSQLiteCache_TTLExpiry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := newTestCache(t)

	if err := c.Set(ctx, "exp", []byte("x"), 80*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, ok := c.Get(ctx, "exp"); !ok {
		t.Fatal("expected hit before expiry")
	}
	time.Sleep(120 * time.Millisecond)
	if _, ok := c.Get(ctx, "exp"); ok {
		t.Fatal("expected miss after TTL")
	}
}

func TestSQLiteCache_NegativeCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := newTestCache(t)

	if c.IsFailed(ctx, "nf") {
		t.Fatal("expected not failed")
	}
	if err := c.SetFailed(ctx, "nf", time.Hour); err != nil {
		t.Fatalf("SetFailed: %v", err)
	}
	if !c.IsFailed(ctx, "nf") {
		t.Fatal("expected failed")
	}
	// Failed entries must not be returned as successful Get hits.
	if _, ok := c.Get(ctx, "nf"); ok {
		t.Fatal("Get must not return failed entries")
	}
}

func TestSQLiteCache_OverwriteSuccessClearsFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := newTestCache(t)

	_ = c.SetFailed(ctx, "k", time.Hour)
	_ = c.Set(ctx, "k", []byte("ok"), time.Hour)
	if c.IsFailed(ctx, "k") {
		t.Fatal("Set should clear failed flag")
	}
	val, ok := c.Get(ctx, "k")
	if !ok || string(val) != "ok" {
		t.Fatalf("got %q ok=%v", val, ok)
	}
}

func newTestCache(t *testing.T) *SQLiteCache {
	t.Helper()
	dir := t.TempDir()
	cfg := &Config{
		MaxEntries:      100,
		SuccessTTL:      shared.Duration{Duration: time.Hour},
		FailureTTL:      shared.Duration{Duration: time.Minute},
		CleanupInterval: shared.Duration{Duration: time.Hour},
	}
	c, err := NewSQLiteCache(filepath.Join(dir, "test.db"), cfg, nil)
	if err != nil {
		t.Fatalf("NewSQLiteCache: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}
