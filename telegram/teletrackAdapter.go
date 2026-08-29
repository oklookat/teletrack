package telegram

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/oklookat/teletrack/core"
)

type TeletrackMessenger struct {
	tg     *TelegramBot
	render *teletrackRenderer

	lastMessageMD5 string
	logger         *slog.Logger
}

func NewTeletrackMessenger(tg *TelegramBot) *TeletrackMessenger {
	if tg == nil {
		return nil
	}

	// Scope logger to this component with context fields
	logger := tg.logger.With(
		slog.String("component", "teletrack_messenger"),
		slog.String("chat_id", tg.cfg.ChatID),
		slog.Int("message_id", tg.cfg.MessageID),
	)

	return &TeletrackMessenger{
		tg:     tg,
		render: &teletrackRenderer{},
		logger: logger,
	}
}

func (t *TeletrackMessenger) UpdatePlaying(ctx context.Context, msg *core.PlayingMessage) error {
	if msg == nil {
		err := errors.New("nil PlayingMessage provided")
		t.logger.ErrorContext(ctx, "failed to update playing message", slog.Any("error", err))
		return err
	}

	params := &bot.EditMessageTextParams{
		ChatID:    t.tg.cfg.ChatID,
		MessageID: t.tg.cfg.MessageID,
		ParseMode: models.ParseModeMarkdown,
		Text:      t.render.BuildMessage(ctx, msg),
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
	}

	// Link preview
	if msg.TrackInfo != nil && msg.TrackInfo.CoverURL() != "" {
		coverURL := msg.TrackInfo.CoverURL()
		params.LinkPreviewOptions = &models.LinkPreviewOptions{
			IsDisabled:       bot.False(),
			PreferLargeMedia: bot.True(),
			URL:              new(coverURL),
		}
	}

	if err := t.editMessageText(ctx, params); err != nil {
		t.logger.ErrorContext(ctx, "failed to update playing status message",
			slog.Any("error", err),
		)
		return fmt.Errorf("update playing status failed: %w", err)
	}

	return nil
}

func (t *TeletrackMessenger) UpdateIdle(ctx context.Context, msg *core.PlayingMessage) error {
	msgStr := t.render.BuildIdleMessage(ctx, msg)

	params := &bot.EditMessageTextParams{
		ChatID:    t.tg.cfg.ChatID,
		MessageID: t.tg.cfg.MessageID,
		ParseMode: models.ParseModeMarkdown,
		Text:      msgStr,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
	}

	// Link preview
	if msg != nil && msg.TrackInfo != nil && msg.TrackInfo.CoverURL() != "" {
		coverURL := msg.TrackInfo.CoverURL()
		params.LinkPreviewOptions = &models.LinkPreviewOptions{
			IsDisabled:       bot.False(),
			PreferLargeMedia: bot.True(),
			URL:              new(coverURL),
		}
	}

	if err := t.editMessageText(ctx, params); err != nil {
		t.logger.ErrorContext(ctx, "failed to update idle status message",
			slog.Any("error", err),
		)
		return fmt.Errorf("update idle status failed: %w", err)
	}

	return nil
}

func (t *TeletrackMessenger) ReportError(ctx context.Context, err error) {
	if err == nil {
		return
	}

	if t.tg == nil || t.tg.bot == nil || t.tg.cfg.ServiceChatID == "" {
		t.logger.WarnContext(ctx, "cannot report error: telegram bot or service chat ID uninitialized",
			slog.Any("reported_error", err),
		)
		return
	}

	if _, sendErr := t.tg.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: t.tg.cfg.ServiceChatID,
		Text:   fmt.Sprintf("Error: %s", err.Error()),
	}); sendErr != nil {
		t.logger.ErrorContext(ctx, "failed to send error report to service chat",
			slog.Any("error", sendErr),
			slog.Any("reported_error", err),
			slog.String("service_chat_id", t.tg.cfg.ServiceChatID),
		)
	}
}

func (t *TeletrackMessenger) editMessageText(ctx context.Context, params *bot.EditMessageTextParams) error {
	if params == nil || params.Text == "" {
		t.logger.DebugContext(ctx, "skipped message edit: empty params or text")
		return nil
	}

	newMsgHash := md5.Sum([]byte(params.Text))
	newMsgHashStr := hex.EncodeToString(newMsgHash[:])

	if t.lastMessageMD5 == newMsgHashStr {
		t.logger.DebugContext(ctx, "skipped message edit: content unchanged",
			slog.String("content_md5", newMsgHashStr),
		)
		return nil
	}

	if _, err := t.tg.bot.EditMessageText(ctx, params); err != nil {
		return err
	}

	t.lastMessageMD5 = newMsgHashStr
	return nil
}
