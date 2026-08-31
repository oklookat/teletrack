package api

import (
	"time"

	"github.com/oklookat/teletrack/core"
)

// State is the public API representation of Teletrack playback state.
//
// This is intentionally separate from core.PlayingMessage so external
// clients have a stable, versionable JSON contract.
//
// Field semantics:
//
//   - Playing is true only while a track is actively playing (not idle/paused-as-idle).
//   - When idle, Track and Artist still describe the last known track when available
//     (same behaviour as the Telegram idle message).
//   - Time is the track/scrobble timestamp when known, otherwise wall clock.
type State struct {
	// Playing is true when Teletrack considers media actively playing.
	Playing bool `json:"playing"`

	// Idle is true when nothing is currently playing (last track may still be set).
	Idle bool `json:"idle"`

	Track  *Track  `json:"track,omitempty"`
	Artist *Artist `json:"artist,omitempty"`

	// Time is the timestamp associated with this state (track time or update time).
	Time time.Time `json:"time"`

	// UpdatedAt is when this snapshot was produced (RFC3339 UTC in JSON).
	UpdatedAt time.Time `json:"updated_at"`

	Emoji         string `json:"emoji,omitempty"`
	Watermark     string `json:"watermark,omitempty"`
	WatermarkLink string `json:"watermark_link,omitempty"`
}

// Track is the public representation of a track.
type Track struct {
	ID string `json:"id"`

	Artist string `json:"artist"`

	Title string `json:"title"`

	TrackLink string `json:"track_link,omitempty"`

	TrackLinkService string `json:"track_link_service,omitempty"`

	CoverURL string `json:"cover_url,omitempty"`

	Playing bool `json:"playing"`

	ProgressMs *int `json:"progress_ms,omitempty"`

	DurationMs *int `json:"duration_ms,omitempty"`

	Time *time.Time `json:"time,omitempty"`
}

// Artist is the public representation of artist information.
type Artist struct {
	Bio string `json:"bio"`

	BioService string `json:"bio_service"`

	Link string `json:"link"`
}

// ErrorResponse is returned for API errors.
type ErrorResponse struct {
	Error string `json:"error"`
}

func stateFromMessage(
	msg *core.PlayingMessage,
	playing bool,
) State {
	now := time.Now().UTC()
	state := State{
		Playing:   playing,
		Idle:      !playing,
		Time:      now,
		UpdatedAt: now,
	}

	if msg == nil {
		return state
	}

	if !msg.Time.IsZero() {
		state.Time = msg.Time.UTC()
	}

	state.Emoji = msg.Emoji
	state.Watermark = msg.Watermark
	state.WatermarkLink = msg.WatermarkLink

	if msg.TrackInfo != nil {
		state.Track = trackFromCore(msg.TrackInfo)
	}

	if msg.ArtistInfo != nil {
		bio := msg.ArtistInfo.Bio()
		link := msg.ArtistInfo.Link()
		service := msg.ArtistInfo.BioService()
		if bio != "" || link != "" || service != "" {
			state.Artist = &Artist{
				Bio:        bio,
				BioService: service,
				Link:       link,
			}
		}
	}

	return state
}

func trackFromCore(track core.Track) *Track {
	result := &Track{
		ID:               track.ID(),
		Artist:           track.Artist(),
		Title:            track.Track(),
		TrackLink:        track.TrackLink(),
		TrackLinkService: track.TrackLinkService(),
		CoverURL:         track.CoverURL(),
		Playing:          track.Playing(),
		ProgressMs:       cloneIntPtr(track.ProgressMs()),
		DurationMs:       cloneIntPtr(track.DurationMs()),
		Time:             cloneTimePtr(track.Time()),
	}

	return result
}

func cloneIntPtr(value *int) *int {
	if value == nil {
		return nil
	}

	result := *value
	return &result
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	result := *value
	return &result
}
