package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/oklookat/teletrack/config"
	"github.com/oklookat/teletrack/lastfm"
	"github.com/oklookat/teletrack/spotify"
)

var (
	_readyToHandle bool
	_tgCfg         *config.Telegram
	_lastFmCfg     *config.LastFm
)

func Boot(ctx context.Context, tgCfg *config.Telegram, lastFmCfg *config.LastFm, spotifyCfg *config.Spotify) error {
	_tgCfg = tgCfg
	_lastFmCfg = lastFmCfg

	_readyToHandle = _tgCfg.UserID > 0 && len(_tgCfg.ServiceChatID) > 0

	opts := []bot.Option{
		bot.WithDefaultHandler(handler),
	}

	b, err := bot.New(_tgCfg.Token, opts...)
	if err != nil {
		return err
	}

	go func() {
		b.Start(ctx)
	}()

	var lastMessage string
	chatID := _tgCfg.ChatID
	messageID := _tgCfg.MessageID

	rMeme := &randomMeme{
		lastMessage: &lastMessage,
		b:           b,
		chatID:      chatID,
		messageID:   messageID,
	}

	lastfmClient := lastfm.NewClient(_lastFmCfg.APIKey)

	spotiCurPlay := &spotifyCurrentPlaying{
		lastfm:      lastfmClient,
		cl:          spotify.GetClient(spotifyCfg.RedirectURI, spotifyCfg.ClientID, spotifyCfg.ClientSecret, spotifyCfg.Token),
		lastMessage: &lastMessage,
		chatID:      chatID,
		messageID:   messageID,
		b:           b,
	}

	lastFmCurPlay := newLastFmCurrentPlaying(
		lastfmClient,
		b,
		chatID,
		messageID,
		_lastFmCfg.Username,
		&lastMessage,
	)

	currentPlayers := []currentPlayer{spotiCurPlay, lastFmCurPlay}

	if !_readyToHandle {
		return err
	}

	for {
		for i := range currentPlayers {
			if err = currentPlayers[i].handle(ctx); err == nil {
				break
			}
			if errors.Is(err, errNothingPlayed) {
				if err := rMeme.handle(ctx); err != nil {
					handleError(ctx, b, fmt.Errorf("randomMeme: %w", err))
				}
				break
			}
			handleError(ctx, b, fmt.Errorf("%s: %w", currentPlayers[i].name(), err))
		}
		time.Sleep(10 * time.Second)
	}
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := getChatIDByUpdate(update)
	userID := getUserIDByUpdate(update)

	var err error
	defer func() {
		if err != nil {
			slog.Error("handler", "err", err)
		}
	}()

	if chatID != nil && userID != nil {
		if !_readyToHandle {
			_, err = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: *chatID,
				Text:   fmt.Sprintf("Telegram user ID: %d\nThis chat ID: %d", update.Message.From.ID, *chatID),
			})
			return
		}
		if _tgCfg.UserID != *userID {
			return
		}
		respMsg := "All good."
		isStop := update.Message.Text == "/stop"
		if isStop {
			respMsg = "Bot stopped."
		}
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: *chatID,
			Text:   respMsg,
		})
		if isStop {
			os.Exit(0)
		}
	}
}

func handleError(ctx context.Context, b *bot.Bot, err error) error {
	_, errd := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: _tgCfg.ServiceChatID,
		Text:   fmt.Sprintf("Error: %s", err.Error()),
	})
	return fmt.Errorf("send: %w, original: %w", errd, err)
}
