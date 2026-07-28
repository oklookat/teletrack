package core

import (
	"time"

	"github.com/oklookat/teletrack/core/lastfm"
	"github.com/oklookat/teletrack/core/spotify"
)

const (
	_watermarkLink = "https://github.com/oklookat/teletrack"
	_watermark     = "powered by oklookat/teletrack"
)

func newPlayingMessage(artistInfo *lastfm.ArtistBio, trackInfo *spotify.Track) PlayingMessage {
	return PlayingMessage{
		ArtistInfo:    artistInfo,
		TrackInfo:     trackInfo,
		Time:          time.Now(),
		Emoji:         totalRandomEmoji(),
		Watermark:     _watermark,
		WatermarkLink: _watermarkLink,
	}
}

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
