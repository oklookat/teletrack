package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/oklookat/teletrack/ago"
	"github.com/oklookat/teletrack/core"
	"github.com/oklookat/teletrack/shared"
)

type teletrackRenderer struct {
	logger *slog.Logger
}

func newTeletrackRenderer(logger *slog.Logger) *teletrackRenderer {
	if logger == nil {
		logger = slog.Default()
	}
	return &teletrackRenderer{
		logger: logger.With(slog.String("component", "teletrack_renderer")),
	}
}

func (t teletrackRenderer) BuildIdleMessage(ctx context.Context, lastPlaying *core.PlayingMessage) string {
	if lastPlaying == nil {
		t.logger.DebugContext(ctx, "building idle message without last track history")
	}

	pMsg := newTeletrackPlayingMessage(ctx, t.logger, lastPlaying)
	return pMsg.BuildIdleMessage()
}

func (t teletrackRenderer) BuildMessage(ctx context.Context, msg *core.PlayingMessage) string {
	if msg == nil {
		t.logger.WarnContext(ctx, "building playing message with nil input")
	}

	pMsg := newTeletrackPlayingMessage(ctx, t.logger, msg)
	return pMsg.BuildPlayingMessage()
}

// Time, status, emoji, watermark not empty, even msg == nil.
func newTeletrackPlayingMessage(ctx context.Context, logger *slog.Logger, msg *core.PlayingMessage) teletrackPlayingMessage {
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
		if msg.TrackInfo.Playing() {
			result.Status = "▶️"
		}

		// Artist - Track.
		artistTrack := fmt.Sprintf("%s - %s", msg.TrackInfo.Artist(), msg.TrackInfo.Track())
		result.ArtistTrack = "`" + sanitizeCodeSpan(artistTrack) + "`"

		// Progress.
		if msg.TrackInfo.ProgressMs() != nil && msg.TrackInfo.DurationMs() != nil {
			progressMs := *msg.TrackInfo.ProgressMs()
			durationMs := *msg.TrackInfo.DurationMs()

			if shared.TrackProgressSupported(&progressMs, &durationMs) {
				formattedProgress := fmt.Sprintf("%s %s %s",
					result.formatTime(progressMs),
					result.formatProgressBar(progressMs, durationMs),
					result.formatTime(durationMs),
				)
				result.Progress = stringPtr(formattedProgress)
			} else {
				logger.DebugContext(ctx, "track progress unsupported or out of bounds",
					slog.Int("progress_ms", progressMs),
					slog.Int("duration_ms", durationMs),
				)
			}
		}

		// Track link.
		if msg.TrackInfo.TrackLink() != "" {
			result.Links = append(result.Links, tgLink("🎹 "+msg.TrackInfo.TrackLinkService(), msg.TrackInfo.TrackLink()))
		}
	} else {
		logger.DebugContext(ctx, "playing message has nil track info")
	}

	if msg.ArtistInfo != nil && msg.ArtistInfo.Bio() != "" {
		// Artist link.
		if msg.ArtistInfo.Link() != "" {
			result.Links = append(result.Links, tgLink("👨‍🎨 "+msg.ArtistInfo.BioService(), msg.ArtistInfo.Link()))
		}

		// Bio.
		if msg.ArtistInfo.Bio() != "" {
			result.Bio = tgText(msg.ArtistInfo.Bio())
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
	Progress    *string
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
		if t.coreMsg != nil {
			b.WriteString(tgText(ago.Format(t.coreMsg.Time)))
		}
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

	// STATUS + trackname
	b.WriteString(t.Status)
	b.WriteString(" ")
	b.WriteString(t.ArtistTrack)
	b.WriteString("\n\n")

	// Progress.
	if t.Progress != nil {
		b.WriteString(*t.Progress)
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
	progressBlocks := int(float64(progressMs) / float64(durationMs) * blocks)
	if progressBlocks > blocks {
		progressBlocks = blocks
	} else if progressBlocks < 0 {
		progressBlocks = 0
	}
	return fmt.Sprintf("[%s%s]", strings.Repeat("█", progressBlocks), strings.Repeat("░", blocks-progressBlocks))
}

func (t teletrackPlayingMessage) formatTime(ms int) string {
	totalSec := ms / 1000
	return fmt.Sprintf("%02d:%02d", totalSec/60, totalSec%60)
}

func stringPtr(s string) *string {
	return &s
}
