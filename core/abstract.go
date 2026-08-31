package core

import (
	"context"
	"time"
)

// Player fetches the currently playing track from a music service.
type Player interface {
	GetPlaying(ctx context.Context) (Track, error)
}

// Renderer pushes status updates to an output channel (Telegram, HTML UI, etc.).
// Implementations must be safe for concurrent use: core may call several
// Renderers in parallel on each tick.
type Renderer interface {
	UpdatePlaying(ctx context.Context, msg *PlayingMessage) error
	UpdateIdle(ctx context.Context, msg *PlayingMessage) error
}

// ArtistGetter fetches short artist biographies from an external service.
type ArtistGetter interface {
	// Service returns a human-readable name, e.g. "Last.fm".
	Service() string

	// GetArtistInfo looks up an artist bio.
	// langs are ISO 639-2 codes; the first language is preferred,
	// the rest are fallbacks when the preferred language has no bio.
	// See https://www.loc.gov/standards/iso639-2/php/code_list.php
	GetArtistInfo(ctx context.Context, artist string, langs []string) (ArtistInfo, error)
}

// ArtistInfo is a short artist biography with a link back to its source.
type ArtistInfo interface {
	// Link is a URL to the artist page or bio on BioService.
	Link() string
	// Bio is a short, cleaned biography text.
	Bio() string
	// BioService is the display name of the source, e.g. "Last.fm".
	BioService() string
}

// Track describes a track that is (or was recently) playing.
type Track interface {
	// ID is a stable identifier, typically an MD5 of "Artist:Track".
	ID() string
	// Playing reports whether the track is actively playing (not paused).
	Playing() bool
	// Artist is the primary artist name.
	Artist() string
	// Track is the track title.
	Track() string
	// TrackLink is an optional URL to the track on the source service.
	TrackLink() string
	// TrackLinkService is the display name for TrackLink, e.g. "Spotify".
	TrackLinkService() string
	// CoverURL is an optional album/track cover image URL.
	CoverURL() string
	// ProgressMs is the current playback position in milliseconds, or nil if unknown.
	ProgressMs() *int
	// DurationMs is the total track length in milliseconds, or nil if unknown.
	DurationMs() *int
	// Time is the timestamp associated with this track (e.g. last scrobble), or nil.
	Time() *time.Time
}

// dummyArtistInfo is a zero-value ArtistInfo used before a real bio is fetched.
type dummyArtistInfo struct{}

func (dummyArtistInfo) Link() string       { return "" }
func (dummyArtistInfo) Bio() string        { return "" }
func (dummyArtistInfo) BioService() string { return "" }
