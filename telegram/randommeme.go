package telegram

import (
	"context"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/oklookat/teletrack/toolbox"
)

type randomMeme struct {
	lastMessage *string

	currentUrl string

	lastUpdated, lastTry time.Time

	b *bot.Bot

	chatID    any
	messageID int

	footerMessage footerMessage
}

func (r *randomMeme) handle(ctx context.Context) error {
	if time.Since(r.lastUpdated) > 10*time.Minute && time.Since(r.lastTry) > 1*time.Minute {
		if err := r.getNewMeme(); err != nil {
			return err
		}
	}

	msg := tgLink("Почти случайный мем каждые 10 минут.", r.currentUrl)

	if r.lastMessage != nil && *r.lastMessage == msg {
		return nil
	}

	_, err := r.b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    r.chatID,
		MessageID: r.messageID,
		ParseMode: models.ParseModeMarkdown,
		Text:      msg + r.footerMessage.update(true),
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled:       bot.False(),
			PreferLargeMedia: bot.True(),
			URL:              &r.currentUrl,
		},
	})
	if err == nil {
		*r.lastMessage = msg
	}

	return err
}

func (r *randomMeme) getNewMeme() error {
	urld, err := toolbox.RandomMemeUrl()
	if err != nil {
		r.lastTry = time.Now()
		return err
	}

	r.currentUrl = urld
	r.lastUpdated = time.Now()
	r.lastTry = time.Now()

	return nil
}
