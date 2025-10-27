package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"github.com/oklookat/teletrack/config"
	"github.com/oklookat/teletrack/module/spotify"

	"github.com/oklookat/teletrack/spoty"
	"github.com/oklookat/teletrack/telegram"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Boot configuration
	if err := config.Boot(); err != nil {
		if strings.Contains(err.Error(), "config created") {
			println(err.Error())
			os.Exit(0)
		}
		slog.Error("config boot failed", "err", err)
		os.Exit(1)
	}

	// Spotify authorization
	if config.C.Spotify.Authorize {
		if err := authorizeSpotify(ctx); err != nil {
			slog.Error("spotify authorization failed", "err", err)
			os.Exit(1)
		}
		slog.Info("Spotify authorization complete")
		return
	}

	spotifyCl := spoty.GetClient(
		config.C.Spotify.RedirectURI,
		config.C.Spotify.ClientID,
		config.C.Spotify.ClientSecret,
		config.C.Spotify.Token,
	)

	// Initialize Telegram bot
	var tgBot *telegram.TelegramBot
	tgBot, err := telegram.NewTelegramBot(ctx, config.C.Telegram, []telegram.Module{
		spotify.NewPlayer(spotifyCl, func(err error) error {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			tgBot.SendError(ctx, err)
			return nil
		}),
	})
	if err != nil {
		slog.Error("failed to start telegram bot", "err", err)
		os.Exit(1)
	}

	// Wait until context is canceled or /stop is received
	select {
	case <-ctx.Done():
	case <-tgBot.StopChannel():
		slog.Info("stop signal received")
	}

	slog.Info("shutting down application")
}

// authorizeSpotify runs OAuth and saves the token
func authorizeSpotify(ctx context.Context) error {
	token, err := spoty.Authorize(ctx, config.C.Spotify, func(url string) {
		slog.Info("Go to URL for Spotify auth", "url", url)
	})
	if err != nil {
		return err
	}
	config.C.Spotify.Authorize = false
	config.C.Spotify.Token = token
	return config.C.Save()
}
