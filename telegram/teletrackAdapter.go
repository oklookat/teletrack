package telegram

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/oklookat/teletrack/core"
)

func NewTeletrackMessenger(tg *TelegramBot) *TeletrackMessenger {
	if tg == nil {
		return nil
	}
	return &TeletrackMessenger{
		tg:     tg,
		render: &teletrackRenderer{},
	}
}

type TeletrackMessenger struct {
	tg     *TelegramBot
	render *teletrackRenderer
}

func (t TeletrackMessenger) UpdatePlaying(ctx context.Context, msg *core.PlayingMessage) error {
	msgStr := t.render.BuildMessage(msg)

	params := &bot.EditMessageTextParams{
		ChatID:    t.tg.cfg.ChatID,
		MessageID: t.tg.cfg.MessageID,
		ParseMode: models.ParseModeMarkdown,
		Text:      msgStr,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
	}

	// Link preview.
	var opts models.LinkPreviewOptions
	if msg.TrackInfo.CoverURL != "" {
		opts = models.LinkPreviewOptions{
			IsDisabled:       bot.False(),
			PreferLargeMedia: bot.True(),
			URL:              &msg.TrackInfo.CoverURL,
		}
		params.LinkPreviewOptions = &opts
	}

	_, err := t.tg.bot.EditMessageText(ctx, params)
	if err != nil {
		return err
	}

	return nil
}

func (t TeletrackMessenger) UpdateIdle(ctx context.Context, msg string) error {
	msgStr := t.render.BuildIdleMessage(msg)

	params := &bot.EditMessageTextParams{
		ChatID:    t.tg.cfg.ChatID,
		MessageID: t.tg.cfg.MessageID,
		ParseMode: models.ParseModeMarkdown,
		Text:      msgStr,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
	}

	_, err := t.tg.bot.EditMessageText(ctx, params)
	if err != nil {
		return err
	}

	return nil
}

func (t TeletrackMessenger) ReportError(ctx context.Context, err error) {
	if _, sendErr := t.tg.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: t.tg.cfg.ServiceChatID,
		Text:   fmt.Sprintf("Error: %s", err.Error()),
	}); sendErr != nil {
		slog.Error("failed to send error message", "err", sendErr)
	}
}
