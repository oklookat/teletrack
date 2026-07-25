package core

import (
	"time"

	"github.com/oklookat/teletrack/core/lastfm"
	"github.com/oklookat/teletrack/core/spotify"
)

type PlayingMessage struct {
	ArtistInfo *lastfm.ArtistBio
	TrackInfo  *spotify.Track

	// Current time.
	Time time.Time

	// Vibe emoji.
	Emoji string

	// "powered by" things.
	Watermark string

	// Link to repo.
	WatermarkLink string
}
