package core

import (
	"crypto/md5"
	"encoding/hex"
	"time"
)

const (
	_watermarkLink = "https://github.com/oklookat/teletrack"
	_watermark     = "powered by oklookat/teletrack"
)

func newPlayingMessage(artistInfo *ArtistInfo, trackInfo *TrackInfo) PlayingMessage {
	msg := PlayingMessage{
		ArtistInfo:    artistInfo,
		TrackInfo:     trackInfo,
		Time:          time.Now(),
		Emoji:         totalRandomEmoji(),
		Watermark:     _watermark,
		WatermarkLink: _watermarkLink,
	}
	if trackInfo != nil {
		trackInfo.GenerateID()
		if trackInfo.Time != nil {
			msg.Time = *trackInfo.Time
		}
	}
	return msg
}

type TrackInfo struct {
	// md5 of "ArtistName:TrackName".
	ID string

	// Playing now?
	Playing bool

	// Artist name.
	Artist string

	// Track name.
	Track string

	// Track link if available. Example: link to Spotify.
	TrackLink string

	// Service name where track link placed. Example: "Spotify".
	TrackLinkService string

	// Track cover URL. Can be emprty.
	CoverURL string

	// Track current progress in ms. Nil if unsupported.
	ProgressMs *int

	// Track total duration in ms. Nil if unsupported.
	DurationMs *int

	// Current time (last track). Nil if unsupported.
	Time *time.Time
}

// Generates ID. Returns result: ID generated or not.
func (t *TrackInfo) GenerateID() bool {
	if t.Artist == "" || t.Track == "" {
		return false
	}
	hash := md5.Sum([]byte(t.Artist + ":" + t.Track))
	t.ID = hex.EncodeToString(hash[:])
	return true
}

func (t TrackInfo) ProgressSupported() bool {
	return t.ProgressMs != nil && t.DurationMs != nil
}

type PlayingMessage struct {
	ArtistInfo *ArtistInfo
	TrackInfo  *TrackInfo

	// Current time.
	Time time.Time

	// Vibe emoji.
	Emoji string

	// "powered by" things.
	Watermark string

	// Link to repo.
	WatermarkLink string
}
