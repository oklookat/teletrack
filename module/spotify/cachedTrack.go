package spotify

import (
	"context"
	"fmt"

	"github.com/oklookat/teletrack/shared"
	"github.com/oklookat/teletrack/spoty"
)

func (s *spotifyPlayerHookImpl) fetchTrackInfo(ctx context.Context, track *spoty.CurrentPlaying) *cachedTrackInfo {
	if cached, ok := s.cachedTracks.Get(track.ID); ok {
		return &cached
	}

	cached := cachedTrackInfo{
		TrackName:   fmt.Sprintf("`%s`", shared.TgText(track.Artist+" - "+track.Name)),
		Popularity:  fmt.Sprintf("🔥 %d / 100", track.FullTrack.Popularity),
		SpotifyLink: fmt.Sprintf("🔗 %s", shared.TgLink("Spotify", "https://open.spotify.com/track/"+track.ID)),
		Emoji:       shared.TgText(shared.TotalRandomEmoji()),
	}

	s.cachedTracks.Add(track.ID, cached)
	return &cached
}

type cachedTrackInfo struct {
	TrackName   string
	Popularity  string
	SpotifyLink string
	Emoji       string
}
