package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/oklookat/teletrack/config"
	"github.com/oklookat/teletrack/core"
	"github.com/oklookat/teletrack/telegram"
	"golang.org/x/oauth2"

	"github.com/oklookat/teletrack/core/lastfm"
	"github.com/oklookat/teletrack/core/spotify"
)

func main() {
	// Flags
	configPath := flag.String("c", "config.json", "config path")
	flag.Parse()

	// Boot configuration
	if err := config.Boot(*configPath); err != nil {
		if strings.Contains(err.Error(), "config created") {
			println(err.Error())
			os.Exit(0)
		}
		chk("config boot failed", err)
	}

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	// Telegram bot
	tgBot, err := telegram.NewTelegramBot(ctx, cancel, config.C.Telegram)
	if err != nil {
		chk("failed to start telegram bot", err)
	}

	// Spotify
	spoty, err := spotify.New(ctx, config.C.Spotify, func(t *oauth2.Token) error {
		config.C.Spotify.Authorize = false
		config.C.Spotify.Token = t
		return config.C.Save()
	})
	if err != nil {
		chk("spotify.New", err)
	}

	// last.fm
	lastFm, err := lastfm.NewClient(config.C.LastFm)
	if err != nil {
		chk("lastfm.NewClient", err)
	}

	tgTeletrack := telegram.NewTeletrackMessenger(tgBot)
	teletrackd := core.New(spoty, lastFm, tgTeletrack, tgTeletrack)

	go func() {
		slog.Info("Starting teletrack...")
		err := teletrackd.Start(ctx)
		if err != nil {
			log.Printf(
				"teletrack stopped: %v",
				err,
			)
		}
	}()

	<-ctx.Done()

	teletrackd.Stop()
}

func chk(msg string, err error) {
	if err == nil {
		return
	}
	slog.Error(msg, "error", err)
	os.Exit(1)
}
