package module

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/oklookat/teletrack/config"
	"github.com/oklookat/teletrack/lastfm"
	"github.com/oklookat/teletrack/shared"
	"github.com/oklookat/teletrack/spotify"

	spotifyapi "github.com/zmb3/spotify/v2"
)

type SpotifyPlayerHooks interface {
	OnNothingPlaying(b *bot.Bot)
	OnNewTrackPlayed(b *bot.Bot, track *spotify.CurrentPlaying)
	OnOldTrackStillPlaying(b *bot.Bot, track *spotify.CurrentPlaying)
}

func NewSpotifyPlayer(cl *spotifyapi.Client) *SpotifySpotifyPlayer {
	return &SpotifySpotifyPlayer{
		cl: cl,
		hooks: &SpotifyPlayerCurrentPlaying{
			lastFmClient: lastfm.NewClient(config.C.LastFm.APIKey),
		},
	}
}

type SpotifySpotifyPlayer struct {
	cl               *spotifyapi.Client
	currentPlaying   *spotify.CurrentPlaying
	lastPlayed       *spotify.CurrentPlaying
	lastProgressIdle int

	hooks SpotifyPlayerHooks

	somethingPlayedBefore bool
}

func (m *SpotifySpotifyPlayer) Handle(ctx context.Context, b *bot.Bot, onError func(error) error) {
	go func() {
		for {
			if err := m.handleReal(ctx, b); err != nil {
				if errd := onError(err); errd != nil {
					return
				}
			}
			if ctx.Err() != nil {
				if err := onError(ctx.Err()); err != nil {
					return
				}
			}
			time.Sleep(10 * time.Second)
		}
	}()
}

func (m *SpotifySpotifyPlayer) handleReal(ctx context.Context, b *bot.Bot) error {
	curPlay, err := spotify.GetCurrentPlaying(ctx, m.cl)
	if err != nil {
		return err
	}

	m.currentPlaying = curPlay

	if ctx.Err() != nil {
		return ctx.Err()
	}

	hooksOk := m.hooks != nil

	if curPlay == nil {
		if hooksOk && m.somethingPlayedBefore {
			m.hooks.OnNothingPlaying(b)
		}
		m.somethingPlayedBefore = false
		return nil
	}

	// Check is nothing played.
	if m.lastPlayed != nil && curPlay.ProgressMs == m.lastPlayed.ProgressMs {
		if m.lastProgressIdle >= 3 {
			if hooksOk && m.somethingPlayedBefore {
				m.hooks.OnNothingPlaying(b)
			}
			m.somethingPlayedBefore = false
			m.lastPlayed = nil
			return nil
		}
		m.lastProgressIdle++
		return nil
	}
	m.lastProgressIdle = 0
	m.somethingPlayedBefore = true

	if m.lastPlayed == nil || m.lastPlayed.ID != m.currentPlaying.ID {
		// New track played.
		if hooksOk {
			m.hooks.OnNewTrackPlayed(b, m.currentPlaying)
		}
	} else {
		// Old track still playing.
		if hooksOk {
			m.hooks.OnOldTrackStillPlaying(b, m.currentPlaying)
		}
	}

	m.lastPlayed = m.currentPlaying

	return nil
}

type SpotifyPlayerCurrentPlaying struct {
	lastFmClient     *lastfm.Client
	lastFmArtistInfo *lastfm.ArtistInfo
	cachedMessage    string
}

func (s SpotifyPlayerCurrentPlaying) OnNothingPlaying(b *bot.Bot) {
	tgMsg := shared.TgText("Nothing is currently playing.")
	s.displayToBot(context.Background(), b, nil, tgMsg)
}

func (s *SpotifyPlayerCurrentPlaying) OnNewTrackPlayed(b *bot.Bot, track *spotify.CurrentPlaying) {
	// Get last fm info.
	lastFmLangs := []string{"en"}
	var lastFmInfo *lastfm.ArtistInfo
	for _, lang := range lastFmLangs {
		laInfo, err := s.lastFmClient.ArtistGetInfo(track.Artist, lang)
		if laInfo == nil || err != nil {
			continue
		}
		bioSum := laInfo.BioSummaryWithoutLinks()
		if len(bioSum) > 0 {
			lastFmInfo = laInfo
			break
		}
		time.Sleep(2 * time.Second)
	}

	s.lastFmArtistInfo = lastFmInfo
	tgMsg := s.format(track, lastFmInfo, true)
	s.displayToBot(context.Background(), b, track, tgMsg)

}

func (s *SpotifyPlayerCurrentPlaying) OnOldTrackStillPlaying(b *bot.Bot, track *spotify.CurrentPlaying) {
	tgMsg := s.format(track, s.lastFmArtistInfo, false)
	s.displayToBot(context.Background(), b, track, tgMsg)
}

func (s *SpotifyPlayerCurrentPlaying) displayToBot(ctx context.Context, b *bot.Bot, track *spotify.CurrentPlaying, msg string) error {
	// Find track cover for preview.
	linkPreview := &models.LinkPreviewOptions{
		IsDisabled: bot.True(),
	}
	if track != nil && track.CoverURL != nil && len(*track.CoverURL) > 0 {
		linkPreview.IsDisabled = bot.False()
		linkPreview.PreferLargeMedia = bot.True()
		linkPreview.URL = track.CoverURL
	}
	// Display to bot.
	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:             config.C.Telegram.ChatID,
		MessageID:          config.C.Telegram.MessageID,
		ParseMode:          models.ParseModeMarkdown,
		Text:               msg,
		LinkPreviewOptions: linkPreview,
	})
	return err
}

func (s *SpotifyPlayerCurrentPlaying) format(track *spotify.CurrentPlaying, bio *lastfm.ArtistInfo, newTrack bool) string {
	progressTime := s.formatTime(track.ProgressMs)
	durationTime := s.formatTime(track.DurationMs)
	progressDurationFormatted := fmt.Sprintf("%s %s %s", progressTime, s.formatProgressBarV1(track.ProgressMs, track.DurationMs), durationTime)
	progressDurationFormatted = shared.TgText(progressDurationFormatted)

	trackName := fmt.Sprintf("`%s`", shared.TgText(shared.EscapeMarkdownV2(s.formatTrackName(track))))
	trackMeta := fmt.Sprintf("Слушаю: %s\n\n%s", trackName, progressDurationFormatted)

	//
	if newTrack {
		var cached string

		if bio != nil {
			artistBio := s.formatArtistBio(bio)
			if len(artistBio) > 0 {
				cached += artistBio + "\n\n"
			}
		}

		// BIO-SPOTIFY-LASTFM
		cached = fmt.Sprintf("%s🔗 %s", cached, shared.TgLink("Spotify", track.Link))
		if bio != nil && len(bio.Artist.URL) > 0 {
			cached += "\n" + shared.TgLink("🔗 Last.fm", bio.Artist.URL)
		}

		cached += "\n\n" + shared.TgText(shared.TotalRandomEmoji())

		s.cachedMessage = cached
	}

	footer := shared.TgLink("powered by oklookat/teletrack", "https://github.com/oklookat/teletrack")
	msg := fmt.Sprintf("%s\n\n%s\n\n%s\n%s", shared.TgText(shared.TimeToRuWithSeconds(time.Now())), trackMeta, s.cachedMessage, footer)

	return msg
}

func (s SpotifyPlayerCurrentPlaying) formatTime(ms int) string {
	minutes := ms / 60000
	seconds := (ms / 1000) % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func (s SpotifyPlayerCurrentPlaying) formatProgressBarV1(progressMs, durationMs int) string {
	totalBlocks := 10 // длина прогресс-бара
	progressBlocks := int(float64(progressMs) / float64(durationMs) * float64(totalBlocks))
	return fmt.Sprintf("[%s%s]", strings.Repeat("█", progressBlocks), strings.Repeat("░", totalBlocks-progressBlocks))
}

func (s SpotifyPlayerCurrentPlaying) formatTrackName(track *spotify.CurrentPlaying) string {
	return track.Artists + " - " + track.Name
}

func (s SpotifyPlayerCurrentPlaying) formatArtistBio(resp *lastfm.ArtistInfo) string {
	if resp == nil || len(resp.Artist.URL) == 0 {
		return ""
	}
	bioSummary := resp.BioSummaryWithoutLinks()
	//bioSummary = trimToFirstNewline(bioSummary)
	bioSummary = shared.RemoveExtraNewlines(bioSummary)
	bioSummary = shared.SliceByRunes(bioSummary, 0, 480)
	//bioSummary = sliceToLastDot(bioSummary)
	bioSummary = strings.TrimSpace(bioSummary)
	if len(bioSummary) < 12 {
		return ""
	}
	// Delete all dots at the end.
	bioSummary = strings.TrimSuffix(bioSummary, ".")
	bioSummary += "..."
	bioSummary = shared.TgText(shared.EscapeMarkdownV2(bioSummary))
	return bioSummary
}
