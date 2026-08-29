package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const (
	DefaultMaxEntries = 10_000
	DefaultSuccessTTL = 7 * 24 * time.Hour
	DefaultFailureTTL = 5 * time.Minute
)

// Cache defines a generic contract for key-value caching operations.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	SetFailed(ctx context.Context, key string, ttl time.Duration) error
	IsFailed(ctx context.Context, key string) bool
	Delete(ctx context.Context, key string) error
	Close() error
}

// SQLiteCache implements the Cache interface backed by SQLite.
type SQLiteCache struct {
	db     *sql.DB
	logger *slog.Logger
	cfg    *Config

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewSQLiteCache initializes a new SQLiteCache instance.
func NewSQLiteCache(dbPath string, cfg *Config, logger *slog.Logger) (*SQLiteCache, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = DefaultMaxEntries
	}
	if cfg.SuccessTTL.Duration <= 0 {
		cfg.SuccessTTL.Duration = DefaultSuccessTTL
	}
	if cfg.FailureTTL.Duration <= 0 {
		cfg.FailureTTL.Duration = DefaultFailureTTL
	}
	if cfg.CleanupInterval.Duration <= 0 {
		cfg.CleanupInterval.Duration = 1 * time.Hour
	}

	if logger == nil {
		logger = slog.Default()
	}

	// Busy timeout and WAL mode configuration
	dsn := dbPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Allow multiple readers in WAL mode, serialize writes via SQLite driver settings
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	ctx, cancel := context.WithCancel(context.Background())

	c := &SQLiteCache{
		db:     db,
		logger: logger,
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	if err := c.migrate(); err != nil {
		_ = db.Close()
		cancel()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	c.startCleanupWorker(cfg.CleanupInterval.Duration)
	c.logger.Info("initialized sqlite cache",
		slog.Int("max_entries", cfg.MaxEntries),
		slog.Duration("success_ttl", cfg.SuccessTTL.Duration),
		slog.Duration("failure_ttl", cfg.FailureTTL.Duration),
		slog.Duration("cleanup_interval", cfg.CleanupInterval.Duration),
	)

	return c, nil
}

func (c *SQLiteCache) migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS kv_cache (
		key        TEXT PRIMARY KEY,
		value      BLOB,
		is_failed  INTEGER NOT NULL DEFAULT 0,
		expires_at INTEGER NOT NULL,
		last_used  INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_kv_cache_lru ON kv_cache(last_used);
	CREATE INDEX IF NOT EXISTS idx_kv_cache_exp ON kv_cache(expires_at);
	`
	_, err := c.db.Exec(query)
	return err
}

func (c *SQLiteCache) Get(ctx context.Context, key string) ([]byte, bool) {
	now := time.Now().Unix()

	var (
		value     []byte
		isFailed  int
		expiresAt int64
	)

	err := c.db.QueryRowContext(ctx, `
		SELECT value, is_failed, expires_at
		FROM kv_cache WHERE key = ?
	`, key).Scan(&value, &isFailed, &expiresAt)

	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			c.logger.ErrorContext(ctx, "failed to query key from cache", slog.String("key", key), slog.Any("error", err))
		}
		return nil, false
	}

	if expiresAt < now || isFailed == 1 {
		return nil, false
	}

	go func() {
		asyncCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = c.db.ExecContext(asyncCtx, `UPDATE kv_cache SET last_used = ? WHERE key = ?`, now, key)
	}()

	return value, true
}

func (c *SQLiteCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.cfg.SuccessTTL.Duration
	}

	now := time.Now().Unix()
	expires := now + int64(ttl.Seconds())

	_, err := c.db.ExecContext(ctx, `
		INSERT INTO kv_cache (key, value, is_failed, expires_at, last_used)
		VALUES (?, ?, 0, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			is_failed = 0,
			expires_at = excluded.expires_at,
			last_used = excluded.last_used
	`, key, value, expires, now)

	if err != nil {
		c.logger.ErrorContext(ctx, "failed to set cache entry", slog.String("key", key), slog.Any("error", err))
		return err
	}

	return nil
}

func (c *SQLiteCache) SetFailed(ctx context.Context, key string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.cfg.FailureTTL.Duration
	}

	now := time.Now().Unix()
	expires := now + int64(ttl.Seconds())

	_, err := c.db.ExecContext(ctx, `
		INSERT INTO kv_cache (key, value, is_failed, expires_at, last_used)
		VALUES (?, NULL, 1, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = NULL,
			is_failed = 1,
			expires_at = excluded.expires_at,
			last_used = excluded.last_used
	`, key, expires, now)

	if err != nil {
		c.logger.ErrorContext(ctx, "failed to set failed entry in cache", slog.String("key", key), slog.Any("error", err))
		return err
	}

	return nil
}

func (c *SQLiteCache) IsFailed(ctx context.Context, key string) bool {
	now := time.Now().Unix()
	var (
		isFailed  int
		expiresAt int64
	)

	err := c.db.QueryRowContext(ctx, `
		SELECT is_failed, expires_at FROM kv_cache WHERE key = ?
	`, key).Scan(&isFailed, &expiresAt)

	if err != nil || expiresAt < now {
		return false
	}

	return isFailed == 1
}

func (c *SQLiteCache) Delete(ctx context.Context, key string) error {
	_, err := c.db.ExecContext(ctx, `DELETE FROM kv_cache WHERE key = ?`, key)
	if err != nil {
		c.logger.ErrorContext(ctx, "failed to delete cache entry", slog.String("key", key), slog.Any("error", err))
		return err
	}
	return nil
}

func (c *SQLiteCache) CleanupExpired(ctx context.Context) {
	now := time.Now().Unix()

	if res, err := c.db.ExecContext(ctx, `DELETE FROM kv_cache WHERE expires_at < ?`, now); err == nil {
		if rows, _ := res.RowsAffected(); rows > 0 {
			c.logger.DebugContext(ctx, "purged expired entries", slog.Int64("count", rows))
		}
	}

	evictQuery := `
		DELETE FROM kv_cache
		WHERE key IN (
			SELECT key FROM kv_cache
			ORDER BY last_used ASC
			LIMIT MAX(0, (SELECT COUNT(*) FROM kv_cache) - ?)
		);
	`
	if res, err := c.db.ExecContext(ctx, evictQuery, c.cfg.MaxEntries); err == nil {
		if rows, _ := res.RowsAffected(); rows > 0 {
			c.logger.InfoContext(ctx, "evicted LRU cache entries", slog.Int64("count", rows))
		}
	}
}

func (c *SQLiteCache) startCleanupWorker(interval time.Duration) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				c.CleanupExpired(c.ctx)
			}
		}
	}()
}

func (c *SQLiteCache) Close() error {
	c.cancel()
	c.wg.Wait()

	if err := c.db.Close(); err != nil {
		c.logger.Error("failed to close sqlite connection", slog.Any("error", err))
		return err
	}

	return nil
}
