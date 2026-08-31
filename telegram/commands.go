package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/oklookat/teletrack/shared"
)

// Stop.

type StopCommand struct{}

func (StopCommand) Command() string { return "/stop" }

func (StopCommand) Help() string { return "Stops program." }

func (StopCommand) Handler(ctx context.Context, b *TelegramBot, args []string) error {
	_, sendErr := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: b.cfg.ServiceChatID,
		Text:   "Program stopped.",
	})
	b.Stop()
	return sendErr
}

// Version.

type VersionCommand struct{}

func (VersionCommand) Command() string { return "/version" }

func (VersionCommand) Help() string { return "Shows installed version." }

func (VersionCommand) Handler(ctx context.Context, b *TelegramBot, args []string) error {
	_, sendErr := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: b.cfg.ServiceChatID,
		Text:   shared.Version,
	})
	return sendErr
}
