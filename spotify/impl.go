package spotify

import (
	"time"

	"github.com/oklookat/teletrack/shared"
)

type TrackInfo struct {
	id                                 string
	spotifyId                          string
	playing                            bool
	artist, track, trackLink, coverURL string
	time                               *time.Time
	progressMs, durationMs             *int
}

func (t *TrackInfo) ID() string {
	if t.id != "" {
		return t.id
	}
	t.id = shared.GenerateTrackID(t.Artist(), t.Track())
	return t.id
}

func (t TrackInfo) Playing() bool {
	return t.playing
}
func (t TrackInfo) Artist() string {
	return t.artist
}
func (t TrackInfo) Track() string {
	return t.track
}
func (t TrackInfo) TrackLink() string {
	return "https://open.spotify.com/track/" + t.spotifyId
}
func (t TrackInfo) TrackLinkService() string {
	return "Spotify"
}
func (t TrackInfo) CoverURL() string {
	return t.coverURL
}
func (t TrackInfo) ProgressMs() *int {
	return t.progressMs
}
func (t TrackInfo) DurationMs() *int {
	return t.durationMs
}
func (t TrackInfo) Time() *time.Time {
	return t.time
}
