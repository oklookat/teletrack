package lastfm

import (
	"time"

	"github.com/oklookat/teletrack/shared"
)

func newArtistInfo(link, bio string) *ArtistInfo {
	return &ArtistInfo{
		link: link,
		bio:  bio,
	}
}

type ArtistInfo struct {
	link string
	bio  string
}

func (a *ArtistInfo) Link() string {
	return a.link
}

func (a *ArtistInfo) Bio() string {
	return a.bio
}

func (a *ArtistInfo) BioService() string {
	return _artistBioService
}

type TrackInfo struct {
	id                                 string
	playing                            bool
	artist, track, trackLink, coverURL string
	time                               *time.Time
}

func (t *TrackInfo) ID() string {
	if t.id != "" {
		return t.id
	}
	t.id = shared.GenerateTrackID(t.Artist(), t.Track())
	return t.id
}

func (t *TrackInfo) Playing() bool {
	return t.playing
}

func (t *TrackInfo) Artist() string {
	return t.artist
}

func (t *TrackInfo) Track() string {
	return t.track
}

func (t *TrackInfo) TrackLink() string {
	return t.trackLink
}

func (t *TrackInfo) TrackLinkService() string {
	return _trackLinkService
}

func (t *TrackInfo) CoverURL() string {
	return t.coverURL
}

func (t *TrackInfo) ProgressMs() *int {
	return nil
}

func (t *TrackInfo) DurationMs() *int {
	return nil
}

func (t *TrackInfo) Time() *time.Time {
	return t.time
}
