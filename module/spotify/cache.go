package spotify

import (
	"context"

	"github.com/oklookat/teletrack/lastfm"
	"github.com/oklookat/teletrack/spoty"
)

func (s *spotifyPlayerHookImpl) fetchArtistInfo(ctx context.Context, track *spoty.CurrentPlaying) *cachedArtistInfo {
	if cached, ok := s.cachedArtists.Get(track.ArtistID); ok {
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

	cached := cachedArtistInfo{}
	cached.format(gotInfo)

	s.cachedArtists.Add(track.ArtistID, cached)
	return &cached
}
