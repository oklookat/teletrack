package telegram

import (
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/oklookat/teletrack/lastfm"
)

func newLastFmCurrentPlaying(client *lastfm.Client, b *bot.Bot, chatID any,
	messageID int,
	username string,
	lastMessage *string) *lastFmCurrentPlaying {
	return &lastFmCurrentPlaying{
		client:      client,
		b:           b,
		chatID:      chatID,
		messageID:   messageID,
		username:    username,
		lastMessage: lastMessage,
	}
}

type lastFmCurrentPlaying struct {
	b *bot.Bot

	client      *lastfm.Client
	chatID      any
	messageID   int
	username    string
	lastMessage *string
}

func (l *lastFmCurrentPlaying) name() string {
	return "lastFmCurrentPlaying"
}

func (l *lastFmCurrentPlaying) handle(ctx context.Context) error {
	tracks, err := l.client.UserGetRecentTracks(l.username, tp(1), nil, nil, tp(true), nil)

	if ctx.Err() != nil {
		return ctx.Err()
	}

	var track *lastfm.Track
	if err == nil && len(tracks.Recenttracks.Track) > 0 {
		track = tracks.Recenttracks.Track[0]
	}

	somethingPlaying := track != nil && track.Attr.Nowplaying != nil && *track.Attr.Nowplaying
	if !somethingPlaying {
		return errNothingPlayed
	}

	artistTrack := tgText(fmt.Sprintf("%s - %s", track.Artist.Name, track.Name))
	if l.lastMessage != nil && artistTrack == *l.lastMessage {
		// Playing same track as before.
		return nil
	}

	var msg string
	myHonestReaction := tgText("My honest reaction: " + totalRandomEmoji())
	updatedAt := tgText("Обновлено: " + timeToRuWithSeconds(time.Now()))
	msg = "Слушаю: " + tgLink(artistTrack, track.URL)
	msg = fmt.Sprintf("%s\n\n%s\n\n%s", msg, myHonestReaction, updatedAt)

	// Find track cover for preview.
	linkPreview := &models.LinkPreviewOptions{
		IsDisabled: bot.True(),
	}
	for _, img := range track.Image {
		if img.Size != "extralarge" {
			continue
		}
		linkPreview.IsDisabled = bot.False()
		linkPreview.PreferLargeMedia = bot.True()
		linkPreview.URL = &img.Text
		break
	}

	_, err = l.b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:             l.chatID,
		MessageID:          l.messageID,
		ParseMode:          models.ParseModeMarkdown,
		Text:               msg,
		LinkPreviewOptions: linkPreview,
	})
	if err == nil {
		*l.lastMessage = artistTrack

	}

	return err
}
