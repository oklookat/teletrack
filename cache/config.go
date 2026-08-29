package cache

import (
	"time"

	"github.com/oklookat/teletrack/shared"
)

type Config struct {
	// MaxEntries is the upper bound for cached items before LRU cleanup kicks in.
	MaxEntries int `json:"maxEntries"`

	// SuccessTTL defines how long valid data stays in cache.
	SuccessTTL shared.Duration `json:"successTTL"`

	// FailureTTL defines how long negative lookup (failed attempts) stays in cache.
	FailureTTL shared.Duration `json:"failureTTL"`

	// CleanupInterval specifies how often the background worker purges expired/LRU records.
	CleanupInterval shared.Duration `json:"cleanupInterval"`
}

func DefaultConfig() *Config {
	return &Config{
		MaxEntries:      DefaultMaxEntries,
		SuccessTTL:      shared.Duration{Duration: DefaultSuccessTTL},
		FailureTTL:      shared.Duration{Duration: DefaultFailureTTL},
		CleanupInterval: shared.Duration{Duration: 1 * time.Hour},
	}
}
