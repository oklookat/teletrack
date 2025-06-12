package telegram

import "github.com/go-telegram/bot/models"

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
