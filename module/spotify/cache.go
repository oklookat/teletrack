package spotify

import (
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/oklookat/teletrack/lastfm"
	"github.com/oklookat/teletrack/shared"
	"github.com/oklookat/teletrack/shared/lastfmclean"
	"github.com/oklookat/teletrack/spoty"
)

const (
	artistInfoFetchTimeout = 5 * time.Second
)

type cachedTrackInfo struct {
	TrackID string
	// TG-formatted
	TrackName          string
	Popularity         string
	Bio                string
	SpotifyLink        string
	LastFmLink         string
	Emoji              string
	LinkPreviewOptions models.LinkPreviewOptions
}

func (s *spotifyPlayerHookImpl) fetchTrackInfo(ctx context.Context, track *spoty.CurrentPlaying) *cachedTrackInfo {
	if cached, ok := s.cachedTracks.Get(track.ArtistID); ok {
		return &cached
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, artistInfoFetchTimeout)
	defer cancel()

	var gotInfo *lastfm.ArtistInfo

	langs := []string{"en", "ru"}
	for _, lang := range langs {
		info, err := s.lastFmClient.ArtistGetInfo(ctxTimeout, track.Artist, lang)
		if err != nil {
			if s.onError != nil {
				s.onError(wrapErr("fetch artist info", err))
			}
			continue
		}
		if info != nil {
			gotInfo = info
			break
		}
	}

	cached := cachedTrackInfo{}
	cached.format(track, gotInfo)

	s.cachedTracks.Add(track.ArtistID, cached)
	return &cached
}

func (a *cachedTrackInfo) format(track *spoty.CurrentPlaying, info *lastfm.ArtistInfo) {
	a.TrackID = track.ID
	a.TrackName = fmt.Sprintf("`%s`", shared.TgText(track.Artist+" - "+track.Name))
	a.Popularity = fmt.Sprintf("🔥 %d / 100", track.FullTrack.Popularity)
	a.Emoji = shared.TgText(shared.TotalRandomEmoji())

	if info != nil {
		a.setBio(info)
		if len(info.Artist.URL) > 0 {
			a.LastFmLink = fmt.Sprintf("🔗 %s", shared.TgLink("Last.fm", info.Artist.URL))
		}
	}

	a.SpotifyLink = fmt.Sprintf("🔗 %s", shared.TgLink("Spotify", "https://open.spotify.com/track/"+track.ID))

	// Link preview.
	opts := models.LinkPreviewOptions{IsDisabled: bot.True()}
	if track.CoverURL != nil && *track.CoverURL != "" {
		opts.IsDisabled = bot.False()
		opts.PreferLargeMedia = bot.True()
		opts.URL = track.CoverURL
	}
	a.LinkPreviewOptions = opts
}

func (a *cachedTrackInfo) setBio(info *lastfm.ArtistInfo) {
	if info == nil {
		return
	}

	cleaner := lastfmclean.NewCleaner(lastfmclean.Config{
		MaxLength:        300,
		RemoveHTML:       true,
		RemoveReferences: true,
		RemoveReadMore:   true,
		ExtractFirstOnly: true,
		RemoveMarkdown:   true,
	})

	bio := cleaner.Clean(info.Artist.Bio.Summary)
	a.Bio = shared.TgText(bio)
}
