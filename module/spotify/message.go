package spotify

import (
	"fmt"
	"strings"
	"time"

	"github.com/oklookat/teletrack/lastfm"
	"github.com/oklookat/teletrack/shared"
	"github.com/oklookat/teletrack/spoty"
)

const (
	bioCacheTTL            = 5 * time.Minute
	artistInfoFetchTimeout = 5 * time.Second
)

func (s *spotifyPlayerHookImpl) formatMessage(info *lastfm.ArtistInfo, playing *spoty.CurrentPlaying) string {
	// base message depending on whether we have bio
	message := s.formatDefaultLinks(playing.ID)
	if info != nil {
		message = s.formatBioAndLinks(info, playing.ID)
	}
	// append emoji footer
	return fmt.Sprintf("%s\n\n%s", message, shared.TgText(shared.TotalRandomEmoji()))
}

func (s *spotifyPlayerHookImpl) buildIdleMessage() string {
	currentTime := shared.TimeToRuWithSeconds(time.Now())
	links := []string{
		shared.TgText("✉️ @dvdqr"),
		shared.TgText("✉️ oklocate@gmail.com"),
		"",
		shared.TgLink("💰 Донат (DA)", "https://donationalerts.com/r/oklookat"),
		shared.TgLink("💰 Донат (Boosty)", "https://boosty.to/oklookat/donate"),
		"",
		shared.TgLink("💻 GitHub", "https://github.com/oklookat"),
		"",
		shared.TgLink("🎧 Spotify", "https://open.spotify.com/user/60c4lc5cwaesypcv9mvzb1klf"),
		shared.TgLink("🎧 Last.fm", "https://last.fm/user/ndskmusic"),
		"",
		shared.TgLink("🍿 Кинопоиск", "https://kinopoisk.ru/user/166758523"),
	}

	var sb strings.Builder
	sb.WriteString(shared.TgText(currentTime))
	sb.WriteString("\n\n")
	sb.WriteString(strings.Join(links, "\n"))
	return sb.String()
}

func (s *spotifyPlayerHookImpl) buildMessage(track *spoty.CurrentPlaying, bio *lastfm.ArtistInfo) string {
	var sb strings.Builder
	sb.WriteString(shared.TgText(shared.TimeToRuWithSeconds(time.Now())))
	sb.WriteString("\n\n")
	sb.WriteString(s.formatTrackInfo(track))

	// bio section may be empty — handle spacing carefully
	bioSection := strings.TrimSpace(s.formatBioSection(track, bio))
	if bioSection != "" {
		sb.WriteString("\n\n")
		sb.WriteString(bioSection)
	}

	// ensure powered-by is on its own paragraph
	sb.WriteString("\n")
	sb.WriteString(shared.TgLink("powered by oklookat/teletrack", "https://github.com/oklookat/teletrack"))
	return sb.String()
}

func (s *spotifyPlayerHookImpl) formatBioSection(track *spoty.CurrentPlaying, bio *lastfm.ArtistInfo) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	cached, ok := s.bioCache[track.ArtistID]

	if ok && cached != nil {
		if cached.playing == nil || cached.playing.ID != track.ID {
			cached.cachedMessage = s.formatMessage(cached.info, track)
			cached.playing = track
		}
		return cached.cachedMessage
	}

	if bio != nil {
		msg := s.formatMessage(bio, track)
		// store the generated message in cache for future
		s.bioCache[track.ArtistID] = &cachedBio{
			info:          bio,
			cachedMessage: msg,
			expiresAt:     time.Now().Add(bioCacheTTL),
			playing:       track,
		}
		return msg
	}

	return s.formatDefaultLinks(track.ID)
}
