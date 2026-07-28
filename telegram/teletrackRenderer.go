package telegram

import (
	"fmt"
	"strings"
	"time"

	"github.com/oklookat/teletrack/core"
	"github.com/oklookat/teletrack/core/ago"
)

type teletrackRenderer struct {
}

func (t teletrackRenderer) BuildIdleMessage(lastPlaying *core.PlayingMessage) string {
	pMsg := newTeletrackPlayingMessage(lastPlaying)
	return pMsg.BuildIdleMessage()
}

func (t teletrackRenderer) BuildMessage(msg *core.PlayingMessage) string {
	pMsg := newTeletrackPlayingMessage(msg)
	return pMsg.BuildPlayingMessage()
}

// Time, status, emoji, watermark not empty, even msg == nil.
func newTeletrackPlayingMessage(msg *core.PlayingMessage) teletrackPlayingMessage {
	result := teletrackPlayingMessage{
		Time:    tgText(timeToRuWithSecondsClockEmoji(time.Now())),
		Status:  "⏸️",
		coreMsg: msg,
	}

	if msg == nil {
		return result
	}

	if msg.TrackInfo != nil {
		// Status.
		if msg.TrackInfo.Playing {
			result.Status = "▶️"
		}

		// Artist - Track.
		artistTrack := fmt.Sprintf("%s - %s", msg.TrackInfo.Artist, msg.TrackInfo.Track)
		result.ArtistTrack = "`" + sanitizeCodeSpan(artistTrack) + "`"

		// Progress.
		result.Progress = fmt.Sprintf("%s %s %s",
			result.formatTime(msg.TrackInfo.ProgressMs),
			result.formatProgressBar(msg.TrackInfo.ProgressMs, msg.TrackInfo.DurationMs),
			result.formatTime(msg.TrackInfo.DurationMs))

		// Track link.
		if msg.TrackInfo.TrackLink != "" {
			result.Links = append(result.Links, tgLink("🔗 Spotify", msg.TrackInfo.TrackLink))
		}
	}

	if msg.ArtistInfo != nil && msg.ArtistInfo.Bio != "" {
		// Artist link.
		if msg.ArtistInfo.Link != "" {
			result.Links = append(result.Links, tgLink("🔗 Last.fm", msg.ArtistInfo.Link))
		}

		// Bio.
		if msg.ArtistInfo.Bio != "" {
			result.Bio = tgText(msg.ArtistInfo.Bio)
		}

	}

	// Emoji.
	if msg.Emoji != "" {
		result.Emoji = tgText(msg.Emoji)
	}

	// Watermark.
	if msg.Watermark != "" {
		result.Watermark = tgLink(msg.Watermark, msg.WatermarkLink)
	}

	return result
}

type teletrackPlayingMessage struct {
	Time        string
	Status      string
	ArtistTrack string
	Progress    string
	Bio         string
	Links       []string
	Emoji       string
	Watermark   string

	coreMsg *core.PlayingMessage
}

func (t teletrackPlayingMessage) BuildIdleMessage() string {
	var b strings.Builder

	// 12:00 12.12.12
	b.WriteString(t.Time)
	b.WriteString("\n\n")

	b.WriteString(tgText("💤 Nothing playing"))
	b.WriteString("\n\n")

	if t.ArtistTrack != "" {
		b.WriteString("┌ Last track · ")
		b.WriteString(tgText(ago.Format(t.coreMsg.Time)))
		b.WriteString("\n")
		b.WriteString("└ ")
		b.WriteString(t.ArtistTrack)
		b.WriteString("\n\n")
	}

	// Bio.
	if t.Bio != "" {
		b.WriteString(t.Bio)
		b.WriteString("\n\n")
	}

	// Links.
	if len(t.Links) > 0 {
		for _, link := range t.Links {
			b.WriteString(link)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Emoji.
	if t.ArtistTrack != "" && t.Emoji != "" {
		b.WriteString(t.Emoji)
		b.WriteString("\n")
	}

	// Watermark.
	if t.Watermark != "" {
		b.WriteString(t.Watermark)
	}

	// Filter.
	return strings.TrimSpace(b.String())
}

func (t teletrackPlayingMessage) BuildPlayingMessage() string {
	var b strings.Builder

	// time
	b.WriteString(t.Time)

	b.WriteString("\n\n")

	// STAUTS + trackname
	b.WriteString(t.Status)
	b.WriteString(" ")
	b.WriteString(t.ArtistTrack)
	b.WriteString("\n\n")

	// Progress.
	b.WriteString(t.Progress)
	b.WriteString("\n\n")

	// Bio.
	if t.Bio != "" {
		b.WriteString(t.Bio)
		b.WriteString("\n\n")
	}

	// Links.
	if len(t.Links) > 0 {
		for _, link := range t.Links {
			b.WriteString(link)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Emoji.
	b.WriteString(t.Emoji)
	b.WriteString("\n")
	// Watermark.
	b.WriteString(t.Watermark)

	// Filter.
	return strings.TrimSpace(b.String())
}

func (t teletrackPlayingMessage) formatProgressBar(progressMs, durationMs int) string {
	const blocks = 12
	if durationMs <= 0 {
		return strings.Repeat("░", blocks)
	}
	progressBlocks := min(int(float64(progressMs)/float64(durationMs)*blocks), blocks)
	return fmt.Sprintf("[%s%s]", strings.Repeat("█", progressBlocks), strings.Repeat("░", blocks-progressBlocks))
}

func (t teletrackPlayingMessage) formatTime(ms int) string {
	totalSec := ms / 1000
	return fmt.Sprintf("%02d:%02d", totalSec/60, totalSec%60)
}
