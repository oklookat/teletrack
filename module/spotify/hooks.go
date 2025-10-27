package spotify

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
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
	lastFmClient *lastfm.Client
	onError      func(error) error

	mu       sync.RWMutex
	bioCache map[string]*cachedBio
	shutdown <-chan struct{}
}

func newSpotifyPlayerHookImpl(lastFmClient *lastfm.Client, onError func(error) error, shutdown <-chan struct{}) *spotifyPlayerHookImpl {
	h := &spotifyPlayerHookImpl{
		lastFmClient: lastFmClient,
		onError:      onError,
		bioCache:     make(map[string]*cachedBio),
		shutdown:     shutdown,
	}
	// background cache cleaner removed in favor of lazy cleanup in getCachedBio
	return h
}

func (s *spotifyPlayerHookImpl) OnNothingPlaying(ctx context.Context, b *bot.Bot) {
	if b == nil {
		return
	}
	msg := s.buildIdleMessage()
	s.sendToBot(ctx, b, nil, msg)
}

func (s *spotifyPlayerHookImpl) OnNewTrackPlayed(ctx context.Context, b *bot.Bot, track *spoty.CurrentPlaying) {
	if b == nil || track == nil {
		return
	}
	bio := s.fetchArtistInfo(ctx, track)
	msg := s.buildMessage(track, bio)
	s.sendToBot(ctx, b, track, msg)
}

func (s *spotifyPlayerHookImpl) OnOldTrackStillPlaying(ctx context.Context, b *bot.Bot, track *spoty.CurrentPlaying) {
	if b == nil || track == nil {
		return
	}
	cached, _ := s.getCachedBio(track.ArtistID)
	var bio *lastfm.ArtistInfo
	if cached != nil {
		bio = cached.info
	}
	msg := s.buildMessage(track, bio)
	s.sendToBot(ctx, b, track, msg)
}

func (s *spotifyPlayerHookImpl) sendToBot(ctx context.Context, b *bot.Bot, track *spoty.CurrentPlaying, msg string) {
	if b == nil {
		return
	}
	// compose link preview options
	opts := &models.LinkPreviewOptions{IsDisabled: bot.True()}
	if track != nil && track.CoverURL != nil && *track.CoverURL != "" {
		opts.IsDisabled = bot.False()
		opts.PreferLargeMedia = bot.True()
		opts.URL = track.CoverURL
	}

	params := &bot.EditMessageTextParams{
		ChatID:             config.C.Telegram.ChatID,
		MessageID:          config.C.Telegram.MessageID,
		ParseMode:          models.ParseModeMarkdown,
		Text:               msg,
		LinkPreviewOptions: opts,
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
