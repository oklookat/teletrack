package spotify

import (
	"context"
	"time"

	"github.com/oklookat/teletrack/lastfm"
	"github.com/oklookat/teletrack/spoty"
)

type cachedBio struct {
	info          *lastfm.ArtistInfo
	playing       *spoty.CurrentPlaying
	expiresAt     time.Time
	cachedMessage string
}

func (s *spotifyPlayerHookImpl) fetchArtistInfo(ctx context.Context, track *spoty.CurrentPlaying) *lastfm.ArtistInfo {
	if cached, found := s.getCachedBio(track.ArtistID); found {
		// refresh cached message for this track id
		cached.cachedMessage = s.formatMessage(cached.info, track)
		return cached.info
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, artistInfoFetchTimeout)
	defer cancel()

	langs := []string{"en", "ru"}
	for _, lang := range langs {
		info, err := s.lastFmClient.ArtistGetInfo(ctxTimeout, track.Artist, lang)
		if err != nil {
			if s.onError != nil {
				s.onError(wrapErr("fetch artist info", err))
			}
			continue
		}
		if info != nil && info.BioSummaryWithoutLinks() != "" {
			s.cacheBio(info, track)
			return info
		}
	}

	s.cacheBio(nil, track)
	return nil
}

func (s *spotifyPlayerHookImpl) getCachedBio(artistId string) (*cachedBio, bool) {
	// lazy cleanup: if expired — delete and return not found
	s.mu.RLock()
	cached, exists := s.bioCache[artistId]
	if !exists {
		s.mu.RUnlock()
		return nil, false
	}
	if time.Now().After(cached.expiresAt) {
		s.mu.RUnlock()
		s.mu.Lock()
		// re-check under write lock
		if cur, ok := s.bioCache[artistId]; ok {
			if time.Now().After(cur.expiresAt) {
				delete(s.bioCache, artistId)
			}
		}
		s.mu.Unlock()
		return nil, false
	}
	// still valid
	s.mu.RUnlock()
	return cached, true
}

func (s *spotifyPlayerHookImpl) cacheBio(info *lastfm.ArtistInfo, playing *spoty.CurrentPlaying) {
	message := s.formatMessage(info, playing)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.bioCache[playing.ArtistID] = &cachedBio{
		info:          info,
		expiresAt:     time.Now().Add(bioCacheTTL),
		cachedMessage: message,
		playing:       playing,
	}
}
