package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Commander interface {
	Command() string
	Handler(ctx context.Context, bot *TelegramBot, args []string) error
}

type TelegramBot struct {
	cfg    *Config
	bot    *bot.Bot
	ready  bool
	cancel context.CancelFunc

	commands map[string]Commander
}

// NewTelegramBot initializes and starts the bot
func NewTelegramBot(ctx context.Context, cancel context.CancelFunc, cfg *Config) (*TelegramBot, error) {
	tg := &TelegramBot{
		cfg:      cfg,
		ready:    cfg.UserID > 0 && len(cfg.ServiceChatID) > 0,
		cancel:   cancel,
		commands: make(map[string]Commander),
	}

	b, err := bot.New(cfg.Token, bot.WithDefaultHandler(tg.defaultHandler))
	if err != nil {
		return nil, err
	}
	tg.bot = b

	tg.RegisterCommand(&StopCommand{})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("telegram bot panic", "err", r)
			}
		}()
		b.Start(ctx)
	}()

	return tg, nil
}

func (tg *TelegramBot) Stop() {
	tg.cancel()
}

func (tg *TelegramBot) RegisterCommand(cmd Commander) {
	tg.commands[cmd.Command()] = cmd
}

func (tg *TelegramBot) defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := getChatIDByUpdate(update)
	userID := getUserIDByUpdate(update)

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

	text := strings.TrimSpace(update.Message.Text)
	if text == "" {
		return
	}

	parts := strings.Fields(text)
	if len(parts) == 0 {
		return
	}
	cmdName := parts[0]
	args := parts[1:]

	respMsg := "Unknown command. Use /help"

	if cmd, ok := tg.commands[cmdName]; ok {
		if err := cmd.Handler(ctx, tg, args); err != nil {
			respMsg = fmt.Sprintf("Exec error %s: %v", cmdName, err)
		} else {
			respMsg = "Done ✅"
		}
	} else if cmdName == "/help" {
		respMsg = tg.helpMessage()
	}

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: *chatID,
		Text:   respMsg,
	}); err != nil {
		slog.Error("failed to send message", "err", err)
	}
}

func (tg *TelegramBot) helpMessage() string {
	var sb strings.Builder
	sb.WriteString("Commands:\n\n")

	for name := range tg.commands {
		sb.WriteString(name)
		sb.WriteString("\n")
	}

	sb.WriteString("\nExample: /wake mycomputer")
	return sb.String()
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
