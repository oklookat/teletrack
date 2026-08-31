package core

import (
	"time"
)

const (
	_watermarkLink = "https://github.com/oklookat/teletrack"
	_watermark     = "powered by oklookat/teletrack"
)

func newPlayingMessage(artistInfo ArtistInfo, trackInfo Track) PlayingMessage {
	msg := PlayingMessage{
		ArtistInfo:    artistInfo,
		TrackInfo:     trackInfo,
		Time:          time.Now(),
		Emoji:         totalRandomEmoji(),
		Watermark:     _watermark,
		WatermarkLink: _watermarkLink,
	}
	if trackInfo != nil {
		if trackInfo.Time() != nil {
			msg.Time = *trackInfo.Time()
		}
	}
	return msg
}

type PlayingMessage struct {
	ArtistInfo ArtistInfo
	TrackInfo  Track

	// Current time.
	Time time.Time

	// Vibe emoji.
	Emoji string

	// "powered by" things.
	Watermark string

	// Link to repo.
	WatermarkLink string
}
