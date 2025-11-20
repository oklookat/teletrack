package spotify

import (
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/oklookat/teletrack/config"
	"github.com/oklookat/teletrack/lastfm"
	"github.com/oklookat/teletrack/spoty"
)

type BotSender interface {
	EditMessageText(ctx context.Context, params *bot.EditMessageTextParams) (*models.Message, error)
}

type SpotifyPlayerHooks interface {
	OnNothingPlaying(ctx context.Context, b *bot.Bot)
	OnNewTrackPlayed(ctx context.Context, b *bot.Bot, track *spoty.CurrentPlaying)
	OnOldTrackStillPlaying(ctx context.Context, b *bot.Bot, track *spoty.CurrentPlaying)
}

type spotifyPlayerHookImpl struct {
	shutdown      <-chan struct{}
	lastFmClient  *lastfm.Client
	onError       func(error) error
	cachedArtists *expirable.LRU[string, cachedArtistInfo]
	cachedTracks  *expirable.LRU[string, cachedTrackInfo]
}

func newSpotifyPlayerHookImpl(lastFmClient *lastfm.Client, onError func(error) error, shutdown <-chan struct{}) *spotifyPlayerHookImpl {
	h := &spotifyPlayerHookImpl{
		lastFmClient:  lastFmClient,
		onError:       onError,
		shutdown:      shutdown,
		cachedArtists: expirable.NewLRU[string, cachedArtistInfo](50, nil, 10*time.Minute),
		cachedTracks:  expirable.NewLRU[string, cachedTrackInfo](50, nil, 10*time.Minute),
	}
	return h
}

func (s *spotifyPlayerHookImpl) OnNothingPlaying(ctx context.Context, b *bot.Bot) {
	if b == nil {
		return
	}
	msg := buildIdleMessage()
	s.sendToBot(ctx, b, nil, msg)
}

func (s *spotifyPlayerHookImpl) OnNewTrackPlayed(ctx context.Context, b *bot.Bot, track *spoty.CurrentPlaying) {
	if b == nil || track == nil {
		return
	}
	artistInfo := s.fetchArtistInfo(ctx, track)
	trackInfo := s.fetchTrackInfo(ctx, track)
	msg := buildPlayingMessage(track, artistInfo, trackInfo)
	s.sendToBot(ctx, b, track, msg)
}

func (s *spotifyPlayerHookImpl) OnOldTrackStillPlaying(ctx context.Context, b *bot.Bot, track *spoty.CurrentPlaying) {
	if b == nil || track == nil {
		return
	}
	artistInfo := s.fetchArtistInfo(ctx, track)
	trackInfo := s.fetchTrackInfo(ctx, track)
	msg := buildPlayingMessage(track, artistInfo, trackInfo)
	s.sendToBot(ctx, b, track, msg)
}

func (s *spotifyPlayerHookImpl) sendToBot(
	ctx context.Context, b *bot.Bot,
	track *spoty.CurrentPlaying,
	msg string,
) {
	if b == nil {
		return
	}

	params := &bot.EditMessageTextParams{
		ChatID:    config.C.Telegram.ChatID,
		MessageID: config.C.Telegram.MessageID,
		ParseMode: models.ParseModeMarkdown,
		Text:      msg,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
	}

	// Link preview.
	var opts models.LinkPreviewOptions
	if track != nil && track.CoverURL != nil && *track.CoverURL != "" {
		opts = models.LinkPreviewOptions{
			IsDisabled:       bot.False(),
			PreferLargeMedia: bot.True(),
			URL:              track.CoverURL,
		}
		params.LinkPreviewOptions = &opts
	}

	var sender BotSender = b
	_, err := sender.EditMessageText(ctx, params)
	if err != nil && s.onError != nil {
		id := ""
		if track != nil {
			id = track.ID
		}
		s.onError(wrapErr(fmt.Sprintf("sendToBot track %s", id), err))
	}
}
