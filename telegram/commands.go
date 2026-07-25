package telegram

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
)

// Stop.

type StopCommand struct{}

func (StopCommand) Command() string { return "/stop" }

func (StopCommand) Handler(ctx context.Context, b *TelegramBot, args []string) error {
	if _, sendErr := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: b.cfg.ServiceChatID,
		Text:   "OK: whole program stopped.",
	}); sendErr != nil {
		slog.Error("failed to send error message", "err", sendErr)
	}
	b.Stop()
	return nil
}
