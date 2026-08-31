package shared

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"time"
)

var Version = "v1.0.0-debug"

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
		return nil
	case float64:
		d.Duration = time.Duration(value)
		return nil
	}
	return nil
}

func TrackProgressSupported(progressMs, durationMs *int) bool {
	return progressMs != nil && durationMs != nil && *durationMs > 0
}

// Generates ID. Returns result: ID generated or empty string.
func GenerateTrackID(artistName, trackName string) string {
	if artistName == "" || trackName == "" {
		return ""
	}
	hash := md5.Sum([]byte(artistName + ":" + trackName))
	return hex.EncodeToString(hash[:])
}

func Unique[T comparable](input []T) []T {
	seen := make(map[T]struct{})
	result := make([]T, 0, len(input))

	for _, val := range input {
		if _, ok := seen[val]; !ok {
			seen[val] = struct{}{}
			result = append(result, val)
		}
	}

	return result
}
