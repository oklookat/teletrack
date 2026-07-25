package telegram

import (
	"fmt"
	"strings"
	"time"

	"github.com/oklookat/teletrack/core"
)

type teletrackRenderer struct {
}

func (t teletrackRenderer) BuildIdleMessage(msg string) string {
	var br strings.Builder

	// time
	br.WriteString(tgText(timeToRuWithSeconds(time.Now())))

	br.WriteString("\n\n")

	br.WriteString(msg)

	return br.String()
}

func (t teletrackRenderer) BuildMessage(msg *core.PlayingMessage) string {
	var br strings.Builder

	// time
	br.WriteString(tgText(timeToRuWithSeconds(msg.Time)))

	br.WriteString("\n\n")

	// STAUTS + trackname
	status := "▶️"
	if !msg.TrackInfo.Playing {
		status = "⏸️"
	}
	artistTrack := fmt.Sprintf("%s - %s", msg.TrackInfo.Artist, msg.TrackInfo.Track)
	artistTrack = status + " `" + sanitizeCodeSpan(artistTrack) + "`"
	br.WriteString(artistTrack)

	br.WriteString("\n\n")

	// Progress.
	progress := fmt.Sprintf("%s %s %s",
		t.formatTime(msg.TrackInfo.ProgressMs),
		t.formatProgressBar(msg.TrackInfo.ProgressMs, msg.TrackInfo.DurationMs),
		t.formatTime(msg.TrackInfo.DurationMs))
	br.WriteString(progress)

	br.WriteString("\n\n")

	// Bio.
	if len(msg.ArtistInfo.Bio) > 0 {
		br.WriteString(tgText(msg.ArtistInfo.Bio))
		br.WriteString("\n\n")
	}

	// Links.
	br.WriteString(tgLink("🔗 Spotify", msg.TrackInfo.TrackLink))
	br.WriteString("\n")
	if len(msg.ArtistInfo.Link) > 0 {
		br.WriteString(tgLink("🔗 Last.fm", msg.ArtistInfo.Link))
		br.WriteString("\n\n")
	} else {
		br.WriteString("\n")
	}

	// Emoji.
	br.WriteString(tgText(msg.Emoji))
	br.WriteString("\n")
	// Watermark.
	br.WriteString(tgLink(msg.Watermark, msg.WatermarkLink))

	return br.String()
}

func (t teletrackRenderer) formatTime(ms int) string {
	totalSec := ms / 1000
	return fmt.Sprintf("%02d:%02d", totalSec/60, totalSec%60)
}

func (t teletrackRenderer) formatProgressBar(progressMs, durationMs int) string {
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
