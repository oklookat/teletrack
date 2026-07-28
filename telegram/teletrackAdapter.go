package telegram

import (
	"context"
	"crypto/md5"
	"encoding/hex"
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

	lastMessageMD5 string
}

func (t *TeletrackMessenger) UpdatePlaying(ctx context.Context, msg *core.PlayingMessage) error {
	params := &bot.EditMessageTextParams{
		ChatID:    t.tg.cfg.ChatID,
		MessageID: t.tg.cfg.MessageID,
		ParseMode: models.ParseModeMarkdown,
		Text:      t.render.BuildMessage(msg),
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

	if err := t.editMessageText(ctx, params); err != nil {
		return err
	}

	return nil
}

func (t *TeletrackMessenger) UpdateIdle(ctx context.Context, msg *core.PlayingMessage) error {
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

	// Link preview.
	var opts models.LinkPreviewOptions
	if msg != nil && msg.TrackInfo != nil && msg.TrackInfo.CoverURL != "" {
		opts = models.LinkPreviewOptions{
			IsDisabled:       bot.False(),
			PreferLargeMedia: bot.True(),
			URL:              &msg.TrackInfo.CoverURL,
		}
		params.LinkPreviewOptions = &opts
	}

	if err := t.editMessageText(ctx, params); err != nil {
		return err
	}

	return nil
}

func (t *TeletrackMessenger) ReportError(ctx context.Context, err error) {
	if _, sendErr := t.tg.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: t.tg.cfg.ServiceChatID,
		Text:   fmt.Sprintf("Error: %s", err.Error()),
	}); sendErr != nil {
		slog.Error("failed to send error message", "err", sendErr)
	}
}

func (t *TeletrackMessenger) editMessageText(ctx context.Context, params *bot.EditMessageTextParams) error {
	if params == nil || params.Text == "" {
		return nil
	}

	newMsgHash := md5.Sum([]byte(params.Text))
	newMsgHashStr := hex.EncodeToString(newMsgHash[:])

	if t.lastMessageMD5 == newMsgHashStr {
		return nil
	}

	_, err := t.tg.bot.EditMessageText(ctx, params)
	if err != nil {
		return err
	}

	t.lastMessageMD5 = newMsgHashStr
	return nil
}
