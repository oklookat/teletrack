package telegram

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/oklookat/teletrack/config"
)

type Module interface {
	Handle(ctx context.Context, b *bot.Bot)
}

type TelegramBot struct {
	cfg    *config.Telegram
	bot    *bot.Bot
	ready  bool
	stopCh chan struct{}
}

// NewTelegramBot initializes and starts the bot
func NewTelegramBot(ctx context.Context, tgCfg *config.Telegram, modules []Module) (*TelegramBot, error) {
	tg := &TelegramBot{
		cfg:    tgCfg,
		ready:  tgCfg.UserID > 0 && len(tgCfg.ServiceChatID) > 0,
		stopCh: make(chan struct{}),
	}

	// Initialize bot with default handler
	b, err := bot.New(tgCfg.Token, bot.WithDefaultHandler(tg.handleInit))
	if err != nil {
		return nil, err
	}
	tg.bot = b

	// Start the bot in a goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("telegram bot panic", "err", r)
			}
		}()
		b.Start(ctx)
	}()

	// Attach modules
	for _, m := range modules {
		m.Handle(ctx, b)
	}

	return tg, nil
}

// handleInit is the default handler for the bot
func (tg *TelegramBot) handleInit(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := getChatIDByUpdate(update)
	userID := getUserIDByUpdate(update)

	if chatID == nil && userID == nil {
		return
	}

	if !tg.ready {
		if chatID != nil {
			_, err := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: *chatID,
				Text:   fmt.Sprintf("Telegram user ID: %d\nService chat ID: %d", update.Message.From.ID, *chatID),
			})
			if err != nil {
				slog.Error("failed to send init message", "err", err)
			}
		}
		return
	}

	if userID == nil || tg.cfg.UserID != *userID {
		return
	}

	respMsg := "All good."
	isStop := update.Message.Text == "/stop"
	if isStop {
		respMsg = "Bot stopped."
	}

	if chatID != nil {
		if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: *chatID,
			Text:   respMsg,
		}); err != nil {
			slog.Error("failed to send message", "err", err)
		}
	}

	if isStop {
		close(tg.stopCh)
	}
}

// SendError sends a message to the service chat about an error
func (tg *TelegramBot) SendError(ctx context.Context, err error) {
	if tg.bot == nil || tg.cfg.ServiceChatID == "" {
		slog.Warn("telegram bot not initialized or service chat ID missing")
		return
	}

	if _, sendErr := tg.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: tg.cfg.ServiceChatID,
		Text:   fmt.Sprintf("Error: %s", err.Error()),
	}); sendErr != nil {
		slog.Error("failed to send error message", "err", sendErr)
	}
}

// StopChannel returns a channel that will be closed when a stop command is received
func (tg *TelegramBot) StopChannel() <-chan struct{} {
	return tg.stopCh
}

func getChatIDByUpdate(update *models.Update) *int64 {
	if update == nil || update.Message == nil {
		return nil
	}
	return &update.Message.Chat.ID
}

func getUserIDByUpdate(update *models.Update) *int64 {
	if update == nil || update.Message == nil || update.Message.From == nil {
		return nil
	}
	return &update.Message.From.ID
}
