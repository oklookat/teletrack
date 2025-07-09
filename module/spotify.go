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
	"github.com/oklookat/teletrack/spoty"

	spotifyapi "github.com/zmb3/spotify/v2"
)

const (
	spotifyRateLimitSec     = 4
	spotifyRateLimit        = spotifyRateLimitSec * time.Second
	spotifyLastProgressIdle = 3 * (spotifyRateLimitSec / 2)
)

type SpotifyPlayerHooks interface {
	OnNothingPlaying(b *bot.Bot)
	OnNewTrackPlayed(b *bot.Bot, track *spoty.CurrentPlaying)
	OnOldTrackStillPlaying(b *bot.Bot, track *spoty.CurrentPlaying)
}

func NewSpotifyPlayer(client *spotifyapi.Client, onError func(error) error) *SpotifyPlayer {
	return &SpotifyPlayer{
		client: client,
		hooks: &spotifyPlayerHookImpl{
			lastFmClient: lastfm.NewClient(config.C.LastFm.APIKey),
			onError:      onError,
		},
		onError: onError,
	}
}

type SpotifyPlayer struct {
	client *spotifyapi.Client

	lastPlayed       *spoty.CurrentPlaying
	lastProgressIdle int

	hooks   SpotifyPlayerHooks
	onError func(error) error
}

func (p *SpotifyPlayer) Handle(ctx context.Context, b *bot.Bot) {
	go func() {
		for {
			if err := p.handle(ctx, b); err != nil && p.onError != nil {
				if err2 := p.onError(err); err2 != nil {
					return
				}
			}

			if ctx.Err() != nil && p.onError != nil {
				if err := p.onError(ctx.Err()); err != nil {
					return
				}
			}

			time.Sleep(spotifyRateLimit)
		}
	}()
}

func (p *SpotifyPlayer) handle(ctx context.Context, b *bot.Bot) error {
	currentPlaying, err := spoty.GetCurrentPlaying(ctx, p.client)
	if err != nil {
		return err
	}

	if p.hooks == nil {
		return nil
	}

	if currentPlaying == nil {
		p.hooks.OnNothingPlaying(b)
		p.lastPlayed = nil
		return nil
	}

	// Track is paused but still the same track playing
	if p.lastPlayed != nil && currentPlaying.ID == p.lastPlayed.ID && !currentPlaying.Playing {
		if p.lastProgressIdle >= spotifyLastProgressIdle {
			p.hooks.OnNothingPlaying(b)
			return nil
		}
		p.lastProgressIdle++
		p.hooks.OnOldTrackStillPlaying(b, currentPlaying)
		return nil
	}

	p.lastProgressIdle = 0

	if p.lastPlayed == nil || p.lastPlayed.ID != currentPlaying.ID {
		p.hooks.OnNewTrackPlayed(b, currentPlaying)
	} else {
		p.hooks.OnOldTrackStillPlaying(b, currentPlaying)
	}

	p.lastPlayed = currentPlaying
	return nil
}

type spotifyPlayerHookImpl struct {
	lastFmClient     *lastfm.Client
	lastFmArtistInfo *lastfm.ArtistInfo
	cachedMessage    string

	onError func(error) error
}

func (s *spotifyPlayerHookImpl) OnNothingPlaying(b *bot.Bot) {
	currentTime := shared.TimeToRuWithSeconds(time.Now())

	message := shared.TgText(currentTime)

	message += "\n\n" + strings.Join([]string{
		shared.TgText("✉️ @dvdqr"),
		shared.TgText("✉️ oklocate@gmail.com"),
		"",
		shared.TgLink("💰 Донат (DA)", "https://donationalerts.com/r/oklookat"),
		shared.TgLink("💰 Донат (Boosty)", "https://boosty.to/oklookat/donate"),
		"",
		shared.TgLink("💻 GitHub", "https://github.com/oklookat"),
		"",
		shared.TgLink("🎧 Spotify", "https://open.spoty.com/user/60c4lc5cwaesypcv9mvzb1klf"),
		shared.TgLink("🎧 Last.fm", "https://last.fm/user/ndskmusic"),
		"",
		shared.TgLink("🍿 Кинопоиск", "https://kinopoisk.ru/user/166758523"),
	}, "\n")

	_ = s.displayToBot(context.Background(), b, nil, message)
}

func (s *spotifyPlayerHookImpl) OnNewTrackPlayed(b *bot.Bot, track *spoty.CurrentPlaying) {
	lastFmInfo := s.fetchArtistInfo(track.Artist)
	s.lastFmArtistInfo = lastFmInfo

	message := s.formatMessage(track, lastFmInfo, true)
	_ = s.displayToBot(context.Background(), b, track, message)
}

func (s *spotifyPlayerHookImpl) fetchArtistInfo(artist string) *lastfm.ArtistInfo {
	langs := []string{"en"}
	for _, lang := range langs {
		info, err := s.lastFmClient.ArtistGetInfo(artist, lang)
		if err != nil && s.onError != nil {
			_ = s.onError(err)
		}
		if info == nil {
			continue
		}
		if bio := info.BioSummaryWithoutLinks(); len(bio) > 0 {
			return info
		}
		time.Sleep(2 * time.Second) // Respect API limits
	}
	return nil
}

func (s *spotifyPlayerHookImpl) OnOldTrackStillPlaying(b *bot.Bot, track *spoty.CurrentPlaying) {
	message := s.formatMessage(track, s.lastFmArtistInfo, false)
	_ = s.displayToBot(context.Background(), b, track, message)
}

func (s *spotifyPlayerHookImpl) displayToBot(ctx context.Context, b *bot.Bot, track *spoty.CurrentPlaying, msg string) error {
	if b == nil {
		return nil
	}

	linkPreview := &models.LinkPreviewOptions{IsDisabled: bot.True()}
	if track != nil && track.CoverURL != nil && len(*track.CoverURL) > 0 {
		linkPreview.IsDisabled = bot.False()
		linkPreview.PreferLargeMedia = bot.True()
		linkPreview.URL = track.CoverURL
	}

	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:             config.C.Telegram.ChatID,
		MessageID:          config.C.Telegram.MessageID,
		ParseMode:          models.ParseModeMarkdown,
		Text:               msg,
		LinkPreviewOptions: linkPreview,
	})
	if err != nil && s.onError != nil {
		s.onError(fmt.Errorf("spotifyPlayerHookImpl, displayToBot, trackID (%s): %w", track.ID, err))
	}

	return err
}

func (s *spotifyPlayerHookImpl) formatMessage(track *spoty.CurrentPlaying, bio *lastfm.ArtistInfo, isNewTrack bool) string {
	progress := fmt.Sprintf("%s %s %s",
		s.formatTime(track.ProgressMs),
		s.formatProgressBar(track.ProgressMs, track.DurationMs),
		s.formatTime(track.DurationMs),
	)

	trackName := fmt.Sprintf("`%s`", shared.TgText(shared.EscapeMarkdownV2(s.formatTrackName(track))))

	statusIcon := "▶️"
	if !track.Playing {
		statusIcon = "⏸️"
	}

	meta := fmt.Sprintf("%s %s", statusIcon, trackName)
	meta += "\n\n" + shared.TgText(progress)
	meta += fmt.Sprintf("\n\n🔥 %d / 100", track.FullTrack.Popularity)

	if isNewTrack {
		var bioText string
		if bio != nil {
			bioText = s.formatArtistBio(bio)
		}

		cached := ""
		if bioText != "" {
			cached = bioText + "\n\n"
		}
		cached += fmt.Sprintf("🔗 %s", shared.TgLink("Spotify", track.Link))
		if bio != nil && bio.Artist.URL != "" {
			cached += "\n" + shared.TgLink("🔗 Last.fm", bio.Artist.URL)
		}
		cached += "\n\n" + shared.TgText(shared.TotalRandomEmoji())

		s.cachedMessage = cached
	}

	footer := shared.TgLink("powered by oklookat/teletrack", "https://github.com/oklookat/teletrack")

	return fmt.Sprintf("%s\n\n%s\n\n%s\n%s",
		shared.TgText(shared.TimeToRuWithSeconds(time.Now())),
		meta,
		s.cachedMessage,
		footer,
	)
}

func (s *spotifyPlayerHookImpl) formatTime(ms int) string {
	minutes := ms / 60000
	seconds := (ms / 1000) % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func (s *spotifyPlayerHookImpl) formatProgressBar(progressMs, durationMs int) string {
	const totalBlocks = 10
	progressBlocks := int(float64(progressMs) / float64(durationMs) * totalBlocks)
	return fmt.Sprintf("[%s%s]",
		strings.Repeat("█", progressBlocks),
		strings.Repeat("░", totalBlocks-progressBlocks),
	)
}

func (s *spotifyPlayerHookImpl) formatTrackName(track *spoty.CurrentPlaying) string {
	return track.Artist + " - " + track.Name
}

func (s *spotifyPlayerHookImpl) formatArtistBio(info *lastfm.ArtistInfo) string {
	if info == nil || info.Artist.URL == "" {
		return ""
	}

	bio := shared.RemoveExtraNewlines(info.BioSummaryWithoutLinks())
	bio = shared.TruncateText(bio, 10, 324)
	bio = strings.TrimSpace(bio)

	if len(bio) < 12 {
		return ""
	}

	return shared.TgText(shared.EscapeMarkdownV2(bio))
}
