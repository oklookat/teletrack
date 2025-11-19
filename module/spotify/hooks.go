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
	shutdown     <-chan struct{}
	lastFmClient *lastfm.Client
	onError      func(error) error
	cachedTracks *expirable.LRU[string, cachedTrackInfo]
}

func newSpotifyPlayerHookImpl(lastFmClient *lastfm.Client, onError func(error) error, shutdown <-chan struct{}) *spotifyPlayerHookImpl {
	h := &spotifyPlayerHookImpl{
		lastFmClient: lastFmClient,
		onError:      onError,
		shutdown:     shutdown,
		cachedTracks: expirable.NewLRU[string, cachedTrackInfo](50, nil, 10*time.Minute),
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
	cached := s.fetchTrackInfo(ctx, track)
	msg := buildPlayingMessage(track, cached)
	s.sendToBot(ctx, b, cached, msg)
}

func (s *spotifyPlayerHookImpl) OnOldTrackStillPlaying(ctx context.Context, b *bot.Bot, track *spoty.CurrentPlaying) {
	if b == nil || track == nil {
		return
	}
	cached := s.fetchTrackInfo(ctx, track)
	msg := buildPlayingMessage(track, cached)
	s.sendToBot(ctx, b, cached, msg)
}

func (s *spotifyPlayerHookImpl) sendToBot(
	ctx context.Context, b *bot.Bot,
	cached *cachedTrackInfo,
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
	if cached != nil {
		params.LinkPreviewOptions = &cached.LinkPreviewOptions
	}

	var sender BotSender = b
	_, err := sender.EditMessageText(ctx, params)
	if err != nil && s.onError != nil {
		id := ""
		if cached != nil {
			id = cached.TrackID
		}
		s.onError(wrapErr(fmt.Sprintf("sendToBot track %s", id), err))
	}
}
