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

const (
	_SPOTIFY_RATE_LIMIT_SEC     = 4
	_SPOTIFY_RATE_LIMIT         = _SPOTIFY_RATE_LIMIT_SEC * time.Second
	_SPOTIFY_LAST_PROGRESS_IDLE = 3 * (_SPOTIFY_RATE_LIMIT_SEC / 2)
)

type SpotifyPlayerHooks interface {
	OnNothingPlaying(b *bot.Bot)
	OnNewTrackPlayed(b *bot.Bot, track *spotify.CurrentPlaying)
	OnOldTrackStillPlaying(b *bot.Bot, track *spotify.CurrentPlaying)
}

func NewSpotifyPlayer(cl *spotifyapi.Client, onError func(error) error) *SpotifyPlayer {
	return &SpotifyPlayer{
		cl: cl,
		hooks: &SpotifyPlayerHookImpl{
			lastFmClient: lastfm.NewClient(config.C.LastFm.APIKey),
			onError:      onError,
		},
		onError: onError,
	}
}

type SpotifyPlayer struct {
	cl *spotifyapi.Client

	lastPlayed       *spotify.CurrentPlaying
	lastProgressIdle int

	hooks SpotifyPlayerHooks

	onError func(error) error
}

func (m *SpotifyPlayer) Handle(ctx context.Context, b *bot.Bot) {
	go func() {
		for {
			if err := m.handleReal(ctx, b); err != nil {
				if m.onError != nil {
					if errd := m.onError(err); errd != nil {
						return
					}
				}
			}
			if ctx.Err() != nil {
				if m.onError != nil {
					if err := m.onError(ctx.Err()); err != nil {
						return
					}
				}
			}
			time.Sleep(_SPOTIFY_RATE_LIMIT)
		}
	}()
}

func (m *SpotifyPlayer) handleReal(ctx context.Context, b *bot.Bot) error {
	curPlay, err := spotify.GetCurrentPlaying(ctx, m.cl)
	if err != nil {
		return err
	}

	hooksOk := m.hooks != nil

	if curPlay == nil {
		if hooksOk {
			m.hooks.OnNothingPlaying(b)
		}
		m.lastPlayed = nil
		return nil
	}

	// Check is nothing played.
	if m.lastPlayed != nil && curPlay.ID == m.lastPlayed.ID && !curPlay.Playing {
		if m.lastProgressIdle >= _SPOTIFY_LAST_PROGRESS_IDLE {
			if hooksOk {
				m.hooks.OnNothingPlaying(b)
			}
			return nil
		}
		m.lastProgressIdle++
		m.hooks.OnOldTrackStillPlaying(b, curPlay)
		return nil
	}

	m.lastProgressIdle = 0

	if m.lastPlayed == nil || m.lastPlayed.ID != curPlay.ID {
		// New track played.
		if hooksOk {
			m.hooks.OnNewTrackPlayed(b, curPlay)
		}
	} else {
		// Old track still playing.
		if hooksOk {
			m.hooks.OnOldTrackStillPlaying(b, curPlay)
		}
	}

	m.lastPlayed = curPlay

	return nil
}

type SpotifyPlayerHookImpl struct {
	lastFmClient     *lastfm.Client
	lastFmArtistInfo *lastfm.ArtistInfo
	cachedMessage    string

	topCached     string
	topLastCached time.Time

	onError func(error) error
}

func (s *SpotifyPlayerHookImpl) OnNothingPlaying(b *bot.Bot) {
	currentTime := shared.TimeToRuWithSeconds(time.Now())

	if time.Since(s.topLastCached) > 5*time.Hour {
		const topMessage = "Топ за неделю:"

		var topTrackMessage string
		topTracks, err := s.lastFmClient.UserGetTopTracks(config.C.LastFm.Username,
			shared.TypeToPtr(lastfm.UserGetTopTracksPeriod7Day),
			shared.TypeToPtr(1), nil)
		if err != nil && s.onError != nil {
			s.onError(err)
		}
		if err == nil && topTracks != nil && len(topTracks.Toptracks.Track) > 0 {
			topTrack := topTracks.Toptracks.Track[0]
			artistName := fmt.Sprintf("`%s - %s`", topTrack.Artist.Name, topTrack.Name)
			topTrackMessage = fmt.Sprintf("Трек (слушал %s): %s", shared.FormatRaz(topTrack.Playcount), artistName)
			topTrackMessage = shared.EscapeMarkdownV2(topTrackMessage)
		}

		var topArtistMessage string
		topArtists, err := s.lastFmClient.UserGetTopArtists(config.C.LastFm.Username,
			shared.TypeToPtr(lastfm.UserGetTopTracksPeriod7Day),
			shared.TypeToPtr(1), nil)
		if err != nil && s.onError != nil {
			s.onError(err)
		}
		if err == nil && topArtists != nil && len(topArtists.Topartists.Artist) > 0 {
			topArtist := topArtists.Topartists.Artist[0]
			topArtistMessage = fmt.Sprintf("Исполнитель (слушал %s): `%s`", shared.FormatRaz(topArtist.Playcount), topArtist.Name)
			topArtistMessage = shared.EscapeMarkdownV2(topArtistMessage)
		}

		s.topCached = topMessage + "\n" + topTrackMessage + "\n" + topArtistMessage
		s.topLastCached = time.Now()
	}

	placeholder := shared.TgText(currentTime)

	if len(s.topCached) > 0 {
		placeholder += "\n\n" + s.topCached
	}

	placeholder = fmt.Sprintf("%s\n\n%s\n%s\n\n%s\n%s\n\n%s\n\n%s\n%s\n\n%s",
		placeholder,
		shared.TgText("✉️ @dvdqr"),
		shared.TgText("✉️ oklocate@gmail.com"),
		shared.TgLink("💰 Донат (DA)", "https://donationalerts.com/r/oklookat"),
		shared.TgLink("💰 Донат (Boosty)", "https://boosty.to/oklookat/donate"),
		shared.TgLink("💻 GitHub", "https://github.com/oklookat"),
		shared.TgLink("🎧 Spotify", "https://open.spotify.com/user/60c4lc5cwaesypcv9mvzb1klf"),
		shared.TgLink("🎧 Last.fm", "https://last.fm/user/ndskmusic"),
		shared.TgLink("🍿 Кинопоиск", "https://kinopoisk.ru/user/166758523"),
	)

	s.displayToBot(context.Background(), b, nil, placeholder)
}

func (s *SpotifyPlayerHookImpl) OnNewTrackPlayed(b *bot.Bot, track *spotify.CurrentPlaying) {
	// Get last fm info.
	lastFmLangs := []string{"en"}
	var lastFmInfo *lastfm.ArtistInfo
	for _, lang := range lastFmLangs {
		laInfo, err := s.lastFmClient.ArtistGetInfo(track.Artist, lang)
		if err != nil && s.onError != nil {
			s.onError(err)
		}
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

func (s *SpotifyPlayerHookImpl) OnOldTrackStillPlaying(b *bot.Bot, track *spotify.CurrentPlaying) {
	tgMsg := s.format(track, s.lastFmArtistInfo, false)
	s.displayToBot(context.Background(), b, track, tgMsg)
}

func (s *SpotifyPlayerHookImpl) displayToBot(ctx context.Context, b *bot.Bot, track *spotify.CurrentPlaying, msg string) error {
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

func (s *SpotifyPlayerHookImpl) format(track *spotify.CurrentPlaying, bio *lastfm.ArtistInfo, newTrack bool) string {
	progressTime := s.formatTime(track.ProgressMs)
	durationTime := s.formatTime(track.DurationMs)
	progressDurationFormatted := fmt.Sprintf("%s %s %s", progressTime, s.formatProgressBarV1(track.ProgressMs, track.DurationMs), durationTime)
	progressDurationFormatted = shared.TgText(progressDurationFormatted)

	trackName := fmt.Sprintf("`%s`", shared.TgText(shared.EscapeMarkdownV2(s.formatTrackName(track))))

	trackListeningStatus := "Слушаю"
	if !track.Playing {
		trackListeningStatus = "На паузе"
	}
	trackMeta := fmt.Sprintf("%s: %s\n\n%s", trackListeningStatus, trackName, progressDurationFormatted)

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

func (s *SpotifyPlayerHookImpl) formatTime(ms int) string {
	minutes := ms / 60000
	seconds := (ms / 1000) % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func (s *SpotifyPlayerHookImpl) formatProgressBarV1(progressMs, durationMs int) string {
	totalBlocks := 10 // длина прогресс-бара
	progressBlocks := int(float64(progressMs) / float64(durationMs) * float64(totalBlocks))
	return fmt.Sprintf("[%s%s]", strings.Repeat("█", progressBlocks), strings.Repeat("░", totalBlocks-progressBlocks))
}

func (s *SpotifyPlayerHookImpl) formatTrackName(track *spotify.CurrentPlaying) string {
	return track.Artist + " - " + track.Name
}

func (s *SpotifyPlayerHookImpl) formatArtistBio(resp *lastfm.ArtistInfo) string {
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
