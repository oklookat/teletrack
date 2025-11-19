package spotify

import (
	"fmt"
	"strings"
	"time"

	"github.com/oklookat/teletrack/shared"
	"github.com/oklookat/teletrack/spoty"
)

func buildPlayingMessage(playing *spoty.CurrentPlaying, info *cachedTrackInfo) string {
	var sb strings.Builder

	// Current time.
	currentTime := shared.TimeToRuWithSeconds(time.Now())
	sb.WriteString(shared.TgText(currentTime) + "\n\n")

	// Track name.
	status := "▶️"
	if !playing.Playing {
		status = "⏸️"
	}
	sb.WriteString(status + " " + info.TrackName + "\n\n")

	// Progress.
	progress := fmt.Sprintf("%s %s %s",
		formatTime(playing.ProgressMs),
		formatProgressBar(playing.ProgressMs, playing.DurationMs),
		formatTime(playing.DurationMs))
	sb.WriteString(progress + "\n\n")

	// Popularity.
	sb.WriteString(info.Popularity + "\n\n")

	// Bio.
	if len(info.Bio) > 0 {
		sb.WriteString(info.Bio + "\n\n")
	}

	// Links.
	sb.WriteString(info.SpotifyLink + "\n")
	if len(info.LastFmLink) > 0 {
		sb.WriteString(info.LastFmLink + "\n\n")
	} else {
		sb.WriteString("\n")
	}

	// Emoji.
	sb.WriteString(info.Emoji + "\n")
	// powered by
	sb.WriteString(shared.TgLink("powered by oklookat/teletrack", "https://github.com/oklookat/teletrack"))

	return sb.String()
}

func buildIdleMessage() string {
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

func formatTime(ms int) string {
	totalSec := ms / 1000
	return fmt.Sprintf("%02d:%02d", totalSec/60, totalSec%60)
}

func formatProgressBar(progressMs, durationMs int) string {
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
