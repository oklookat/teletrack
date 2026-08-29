package shared

import (
	"crypto/md5"
	"encoding/hex"
)

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
