// Package shared provides cross-package utilities used throughout teletrack.
package shared

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Version is set at build time via -ldflags.
// Example: go build -ldflags "-X github.com/oklookat/teletrack/shared.Version=v1.2.3"
var Version = "v1.0.0-debug"

// Duration wraps time.Duration so it can be unmarshaled from JSON as either
// a string duration ("1h30m") or a numeric nanosecond count.
type Duration struct {
	time.Duration
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		parsed, err := time.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("parse duration %q: %w", s, err)
		}

		d.Duration = parsed
		return nil
	}

	var ns int64
	if err := json.Unmarshal(b, &ns); err == nil {
		d.Duration = time.Duration(ns)
		return nil
	}

	return fmt.Errorf("duration must be a string or integer nanoseconds")
}

// MarshalJSON implements json.Marshaler.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

// Ptr returns a pointer to v. Useful for optional API parameters.
func Ptr[T any](v T) *T {
	return &v
}

// TrackProgressSupported reports whether both progress and duration are
// present and the track has a positive length.
func TrackProgressSupported(progressMs, durationMs *int) bool {
	return progressMs != nil && durationMs != nil && *durationMs > 0
}

// GenerateTrackID returns a stable MD5 hex digest of "artist:track",
// or an empty string when either name is missing.
func GenerateTrackID(artistName, trackName string) string {
	if artistName == "" || trackName == "" {
		return ""
	}
	hash := md5.Sum([]byte(artistName + ":" + trackName))
	return hex.EncodeToString(hash[:])
}

// Unique returns a new slice with duplicate elements removed,
// preserving first-occurrence order.
func Unique[T comparable](input []T) []T {
	if len(input) == 0 {
		return nil
	}
	seen := make(map[T]struct{}, len(input))
	result := make([]T, 0, len(input))
	for _, val := range input {
		if _, ok := seen[val]; ok {
			continue
		}
		seen[val] = struct{}{}
		result = append(result, val)
	}
	return result
}
