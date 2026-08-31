package listenbrainz

import (
	"context"
	"time"

	"github.com/oklookat/teletrack/core"
	"github.com/oklookat/teletrack/shared"
)

type ArtistGetter struct {
	client *Client
}

func NewArtistGetter(client *Client) core.ArtistGetter {
	return &ArtistGetter{
		client: client,
	}
}

func (ArtistGetter) Service() string {
	return _artistBioService
}

func (g *ArtistGetter) GetArtistInfo(
	ctx context.Context,
	artist string,
	languages []string,
) (core.ArtistInfoer, error) {
	return g.client.GetArtistInfo(ctx, artist, languages)
}

func newArtistInfo(link, bio string) *ArtistInfo {
	return &ArtistInfo{
		link:       link,
		bio:        bio,
		bioService: _artistBioService,
	}
}

type ArtistInfo struct {
	link       string
	bio        string
	bioService string
}

// Link to artist or artist bio on BioService.
func (a ArtistInfo) Link() string {
	return a.link
}

// Artist bio. Must be short, cleaned.
func (a ArtistInfo) Bio() string {
	return a.bio
}

// Example: Last.fm
func (a ArtistInfo) BioService() string {
	return a.bioService
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
	return t.trackLink
}
func (t TrackInfo) TrackLinkService() string {
	return _trackLinkService
}
func (t TrackInfo) CoverURL() string {
	return t.coverURL
}
func (t TrackInfo) ProgressMs() *int {
	return nil
}
func (t TrackInfo) DurationMs() *int {
	return nil
}
func (t TrackInfo) Time() *time.Time {
	return t.time
}
