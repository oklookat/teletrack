package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/oklookat/teletrack/config"
)

var (
	_readyToHandle bool
	_tgCfg         *config.Telegram
	_bot           *bot.Bot
)

type Module interface {
	Handle(ctx context.Context, b *bot.Bot)
}

func Boot(ctx context.Context, tgCfg *config.Telegram, modules []Module) error {
	_tgCfg = tgCfg
	_readyToHandle = _tgCfg.UserID > 0 && len(_tgCfg.ServiceChatID) > 0

	// Init & start bot.
	opts := []bot.Option{
		bot.WithDefaultHandler(handleInit),
	}
	b, err := bot.New(_tgCfg.Token, opts...)
	if err != nil {
		return err
	}
	go func() {
		b.Start(ctx)
	}()
	_bot = b

	for i := range modules {
		modules[i].Handle(ctx, b)
	}

	return err
}

func handleInit(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := getChatIDByUpdate(update)
	userID := getUserIDByUpdate(update)

	var err error
	defer func() {
		if err != nil {
			slog.Error("handler", "err", err)
		}
	}()

	if chatID == nil && userID == nil {
		return
	}

	if !_readyToHandle {
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: *chatID,
			Text:   fmt.Sprintf("Telegram user ID: %d\nService chat ID: %d", update.Message.From.ID, *chatID),
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

func HandleError(ctx context.Context, err error) error {
	_bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: _tgCfg.ServiceChatID,
		Text:   fmt.Sprintf("Error: %s", err.Error()),
	})
	return nil
}
