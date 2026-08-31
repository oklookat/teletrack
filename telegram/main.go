package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Commander interface {
	Command() string
	Help() string
	Handler(ctx context.Context, bot *TelegramBot, args []string) error
}

type TelegramBot struct {
	cfg    *Config
	bot    *bot.Bot
	ready  bool
	cancel context.CancelFunc

	commands map[string]Commander
	logger   *slog.Logger
}

// NewTelegramBot initializes and starts the bot
func NewTelegramBot(ctx context.Context, cancel context.CancelFunc, cfg *Config) (*TelegramBot, error) {
	tg := &TelegramBot{
		cfg:      cfg,
		ready:    cfg.UserID > 0 && len(cfg.ServiceChatID) > 0,
		cancel:   cancel,
		commands: make(map[string]Commander),
		// Scoped logger with component context
		logger: slog.Default().With(slog.String("component", "telegram_bot")),
	}

	b, err := bot.New(cfg.Token,
		bot.WithDefaultHandler(tg.defaultHandler),
		bot.WithHTTPClient(60*time.Second, newTelegramHTTPClient(tg.logger)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot instance: %w", err)
	}
	tg.bot = b

	tg.RegisterCommand(&StopCommand{})
	tg.RegisterCommand(&VersionCommand{})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				tg.logger.ErrorContext(ctx, "telegram bot background worker panicked",
					slog.Any("panic", r),
					slog.String("stack", string(debug.Stack())),
				)
			}
		}()

		tg.logger.InfoContext(ctx, "starting telegram bot polling loop")
		b.Start(ctx)
	}()

	return tg, nil
}

func (tg *TelegramBot) Stop() {
	tg.logger.Info("stopping telegram bot")
	tg.cancel()
}

func (tg *TelegramBot) RegisterCommand(cmd Commander) {
	name := cmd.Command()
	tg.commands[name] = cmd
	tg.logger.Debug("registered command", slog.String("command", name))
}

func (tg *TelegramBot) defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := getChatIDByUpdate(update)
	userID := getUserIDByUpdate(update)

	// Enrich context-aware logging fields
	l := tg.logger
	if chatID != nil {
		l = l.With(slog.Int64("chat_id", *chatID))
	}
	if userID != nil {
		l = l.With(slog.Int64("user_id", *userID))
	}

	if !tg.ready {
		if chatID != nil {
			_, err := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: *chatID,
				Text:   fmt.Sprintf("Telegram user ID: %d\nService chat ID: %d", update.Message.From.ID, *chatID),
			})
			if err != nil {
				l.ErrorContext(ctx, "failed to send initialization message", slog.Any("error", err))
			}
		} else {
			l.WarnContext(ctx, "bot is not ready and update has no chat ID")
		}
		return
	}

	if userID == nil || tg.cfg.UserID != *userID {
		l.WarnContext(ctx, "unauthorized message attempt ignored")
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

	cmdLogger := l.With(slog.String("command", cmdName))
	respMsg := "Unknown command. Use /help"

	if cmd, ok := tg.commands[cmdName]; ok {
		cmdLogger.DebugContext(ctx, "executing command", slog.Int("args_count", len(args)))

		if err := cmd.Handler(ctx, tg, args); err != nil {
			cmdLogger.ErrorContext(ctx, "command execution failed", slog.Any("error", err))
			respMsg = fmt.Sprintf("Exec error %s: %v", cmdName, err)
		}
		return
	} else if cmdName == "/help" {
		respMsg = tg.helpMessage()
	} else {
		cmdLogger.WarnContext(ctx, "unknown command received")
	}

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: *chatID,
		Text:   respMsg,
	}); err != nil {
		l.ErrorContext(ctx, "failed to send response message", slog.Any("error", err))
	}
}

func (tg *TelegramBot) helpMessage() string {
	var sb strings.Builder
	sb.WriteString("Commands:\n\n")

	for name, cmd := range tg.commands {
		sb.WriteString(name)
		if cmd.Help() != "" {
			sb.WriteString(" - ")
			sb.WriteString(cmd.Help())
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// SendError sends a message to the service chat about an error
func (tg *TelegramBot) SendError(ctx context.Context, err error) {
	if err == nil {
		return
	}

	if tg.bot == nil || tg.cfg.ServiceChatID == "" {
		tg.logger.WarnContext(ctx, "cannot send error message: bot uninitialized or service chat ID missing",
			slog.Any("original_error", err),
		)
		return
	}

	if _, sendErr := tg.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: tg.cfg.ServiceChatID,
		Text:   fmt.Sprintf("Error: %s", err.Error()),
	}); sendErr != nil {
		tg.logger.ErrorContext(ctx, "failed to send error message to service chat",
			slog.Any("error", sendErr),
			slog.Any("original_error", err),
			slog.String("service_chat_id", tg.cfg.ServiceChatID),
		)
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

func telegramTransport(logger *slog.Logger) *http.Transport {
	dialer := &net.Dialer{}

	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,

		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Helper to log successful connection metadata
			logSuccess := func(conn net.Conn, networkType string) {
				remoteAddr := conn.RemoteAddr().String()
				host, port, _ := net.SplitHostPort(remoteAddr)

				logger.InfoContext(ctx, "telegram connection established",
					slog.String("network", networkType),
					slog.String("remote_ip", host),
					slog.String("remote_port", port),
					slog.String("target_addr", addr),
				)
			}

			// IPv4: short try.
			v4ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
			defer cancel()

			conn, err4 := dialer.DialContext(v4ctx, "tcp4", addr)
			if err4 == nil {
				logSuccess(conn, "ipv4")
				return conn, nil
			}

			logger.WarnContext(ctx, "ipv4 connection attempt failed, falling back to ipv6",
				slog.Any("error", err4),
				slog.String("target_addr", addr),
			)

			// IPv6: use remaining timeout from the original context.
			conn, err6 := dialer.DialContext(ctx, "tcp6", addr)
			if err6 == nil {
				logSuccess(conn, "ipv6")
				return conn, nil
			}

			err := fmt.Errorf("telegram connection failed: ipv4: %w; ipv6: %v", err4, err6)
			logger.ErrorContext(ctx, "all connection attempts failed",
				slog.String("target_addr", addr),
				slog.Any("error", err),
			)

			return nil, err
		},

		ForceAttemptHTTP2: true,
	}
}

func newTelegramHTTPClient(logger *slog.Logger) *http.Client {
	return &http.Client{
		Transport: telegramTransport(logger),
	}
}
