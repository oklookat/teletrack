package spotify

import (
	"fmt"
	"strings"

	"github.com/oklookat/teletrack/lastfm"
	"github.com/oklookat/teletrack/shared"
	"github.com/oklookat/teletrack/spoty"
)

func (s *spotifyPlayerHookImpl) formatTrackInfo(track *spoty.CurrentPlaying) string {
	status := "▶️"
	if !track.Playing {
		status = "⏸️"
	}
	trackName := fmt.Sprintf("`%s`", shared.TgText(shared.EscapeMarkdownV2(track.Artist+" - "+track.Name)))
	progress := fmt.Sprintf("%s %s %s",
		s.formatTime(track.ProgressMs),
		s.formatProgressBar(track.ProgressMs, track.DurationMs),
		s.formatTime(track.DurationMs))

	return fmt.Sprintf("%s %s\n\n%s\n\n🔥 %d / 100",
		status, trackName, shared.TgText(progress), track.FullTrack.Popularity)
}

func (s *spotifyPlayerHookImpl) formatTime(ms int) string {
	totalSec := ms / 1000
	return fmt.Sprintf("%02d:%02d", totalSec/60, totalSec%60)
}

func (s *spotifyPlayerHookImpl) formatProgressBar(progressMs, durationMs int) string {
	const blocks = 12
	if durationMs <= 0 {
		return strings.Repeat("░", blocks)
	}
	progressBlocks := int(float64(progressMs) / float64(durationMs) * blocks)
	if progressBlocks > blocks {
		progressBlocks = blocks
	}
	return fmt.Sprintf("[%s%s]", strings.Repeat("█", progressBlocks), strings.Repeat("░", blocks-progressBlocks))
}

func (s *spotifyPlayerHookImpl) formatBioAndLinks(info *lastfm.ArtistInfo, trackId string) string {
	if info == nil {
		return s.formatDefaultLinks(trackId)
	}

	var sb strings.Builder
	if bioText := s.formatArtistBio(info); bioText != "" {
		sb.WriteString(bioText)
		sb.WriteString("\n\n")
	}
	sb.WriteString(fmt.Sprintf("🔗 %s", shared.TgLink("Spotify", "https://open.spotify.com/track/"+trackId)))
	if info.Artist.URL != "" {
		sb.WriteString("\n🔗 ")
		sb.WriteString(shared.TgLink("Last.fm", info.Artist.URL))
	}
	return strings.TrimSpace(sb.String())
}

func (s *spotifyPlayerHookImpl) formatDefaultLinks(trackId string) string {
	return fmt.Sprintf("🔗 %s", shared.TgLink("Spotify", "https://open.spotify.com/track/"+trackId))
}

func (s *spotifyPlayerHookImpl) formatArtistBio(info *lastfm.ArtistInfo) string {
	if info == nil {
		return ""
	}
	bio := strings.Join(strings.Fields(info.BioSummaryWithoutLinks()), " ")
	if len(bio) < 20 {
		return ""
	}
	bio = smartTruncateSentences(bio, 300)
	return shared.TgText(shared.EscapeMarkdownV2(bio))
}
