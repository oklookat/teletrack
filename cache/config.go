package cache

import "time"

type Config struct {
	// MaxEntries is the upper bound for cached items before LRU cleanup kicks in.
	MaxEntries int `json:"maxEntries"`

	// SuccessTTL defines how long valid data stays in cache.
	SuccessTTL time.Duration `json:"successTTL"`

	// FailureTTL defines how long negative lookup (failed attempts) stays in cache.
	FailureTTL time.Duration `json:"failureTTL"`

	// CleanupInterval specifies how often the background worker purges expired/LRU records.
	CleanupInterval time.Duration `json:"cleanupInterval"`
}

func DefaultConfig() *Config {
	return &Config{
		MaxEntries:      DefaultMaxEntries,
		SuccessTTL:      DefaultSuccessTTL,
		FailureTTL:      DefaultFailureTTL,
		CleanupInterval: 1 * time.Hour,
	}
}
