package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/oklookat/teletrack/lastfm"
	"github.com/oklookat/teletrack/spotify"
	spotifyapi "github.com/zmb3/spotify/v2"
)

type spotifyCurrentPlaying struct {
	b      *bot.Bot
	cl     *spotifyapi.Client
	lastfm *lastfm.Client

	chatID    any
	messageID int

	lastMessage *string

	lastProgressIdle int

	lastPlayed     *spotify.CurrentPlaying
	currentPlaying *spotify.CurrentPlaying

	currentPlayingArtistInfo *lastfm.ArtistInfo
	currentPlayingArtistBio  string

	footerMessage footerMessage
}

func (l *spotifyCurrentPlaying) name() string {
	return "spotifyCurrentPlaying"
}

func (l *spotifyCurrentPlaying) handle(ctx context.Context) error {
	curPlay, err := spotify.GetCurrentPlaying(ctx, l.cl)
	if err != nil {
		return err
	}
	l.currentPlaying = curPlay

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if curPlay == nil {
		return errNothingPlayed
	}

	// Check is nothing played.
	if l.lastPlayed != nil && curPlay.ProgressMs == l.lastPlayed.ProgressMs {
		if l.lastProgressIdle >= 3 {
			return errNothingPlayed
		}
		l.lastProgressIdle++
		return nil
	}
	l.lastProgressIdle = 0

	l.footerMessage.update(false)

	// New track played.
	if l.lastPlayed == nil || l.lastPlayed.ID != l.currentPlaying.ID {
		l.footerMessage.update(true)

		// Get artist info with fallback to another language.
		langs := []string{"ru", "en"}
		var nothingFound bool
		for _, lang := range langs {
			artistInfo, err := l.lastfm.ArtistGetInfo(curPlay.Artist, lang)
			if err == nil && artistInfo != nil && len(artistInfo.BioSummaryWithoutLinks()) > 12 {
				nothingFound = false
				l.currentPlayingArtistInfo = artistInfo
				l.currentPlayingArtistBio = l.formatArtistBio(l.currentPlayingArtistInfo)
				break
			}
			nothingFound = true
			time.Sleep(2 * time.Second)
		}
		if nothingFound {
			l.currentPlayingArtistBio = ""
			l.currentPlayingArtistInfo = nil
		}
	}

	var lastFmLink string
	if l.currentPlayingArtistInfo != nil {
		lastFmLink = l.currentPlayingArtistInfo.Artist.URL
	}
	progress := l.formatCurrentPlayingV1(curPlay, lastFmLink)

	// Main message.
	artistBioBlock := ""
	if len(l.currentPlayingArtistBio) > 12 {
		artistBioBlock = "\n\n" + l.currentPlayingArtistBio
	}
	msg := progress + artistBioBlock + l.footerMessage.get()

	// Find track cover for preview.
	linkPreview := &models.LinkPreviewOptions{
		IsDisabled: bot.True(),
	}
	if curPlay.CoverURL != nil && len(*curPlay.CoverURL) > 0 {
		linkPreview.IsDisabled = bot.False()
		linkPreview.PreferLargeMedia = bot.True()
		linkPreview.URL = curPlay.CoverURL
	}

	// Display to bot.
	_, err = l.b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:             l.chatID,
		MessageID:          l.messageID,
		ParseMode:          models.ParseModeMarkdown,
		Text:               msg,
		LinkPreviewOptions: linkPreview,
	})
	if err == nil {
		*l.lastMessage = progress
		l.lastPlayed = curPlay
	}

	return err
}

func (l *spotifyCurrentPlaying) formatTime(ms int) string {
	minutes := ms / 60000
	seconds := (ms / 1000) % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func (l *spotifyCurrentPlaying) formatProgressBarV1(progressMs, durationMs int) string {
	totalBlocks := 10 // длина прогресс-бара
	progressBlocks := int(float64(progressMs) / float64(durationMs) * float64(totalBlocks))
	return fmt.Sprintf("[%s%s]", strings.Repeat("█", progressBlocks), strings.Repeat("░", totalBlocks-progressBlocks))
}

func (l *spotifyCurrentPlaying) formatCurrentPlayingV1(track *spotify.CurrentPlaying, lastFmLink string) string {
	progressTime := l.formatTime(track.ProgressMs)
	durationTime := l.formatTime(track.DurationMs)
	progressDurationFormatted := fmt.Sprintf("%s %s %s", progressTime, l.formatProgressBarV1(track.ProgressMs, track.DurationMs), durationTime)
	progressDurationFormatted = tgText(progressDurationFormatted)

	trackName := fmt.Sprintf("`%s`", tgText(escapeMarkdownV2(l.formatTrackName(track))))

	trackMeta := fmt.Sprintf("🎵 %s\n\n%s", trackName, progressDurationFormatted)

	msg := fmt.Sprintf("%s\n\n🔗 %s", trackMeta, tgLink("Spotify", track.Link))
	if len(lastFmLink) > 0 {
		msg += "\n" + tgLink("🔗 Last.fm", lastFmLink)
	}
	return msg
}

func (l *spotifyCurrentPlaying) formatTrackName(track *spotify.CurrentPlaying) string {
	return track.Artists + " - " + track.Name
}

func (l *spotifyCurrentPlaying) formatArtistBio(resp *lastfm.ArtistInfo) string {
	if resp == nil || len(resp.Artist.URL) == 0 {
		return ""
	}
	bioSummary := resp.BioSummaryWithoutLinks()
	//bioSummary = trimToFirstNewline(bioSummary)
	bioSummary = removeExtraNewlines(bioSummary)
	bioSummary = sliceByRunes(bioSummary, 0, 480)
	//bioSummary = sliceToLastDot(bioSummary)
	bioSummary = strings.TrimSpace(bioSummary)
	if len(bioSummary) < 12 {
		return ""
	}
	lastSym := bioSummary[len(bioSummary)-1:]
	if lastSym != "." {
		bioSummary += "."
	}
	bioSummary = tgText(escapeMarkdownV2(bioSummary))
	return bioSummary
}
