package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/oklookat/teletrack/config"
	"github.com/oklookat/teletrack/core"
	"github.com/oklookat/teletrack/loader"
	"github.com/oklookat/teletrack/telegram"
)

var version = "1.0.0"

func main() {
	slog.Info("teletrack", "version", version)

	// Flags
	configPath := flag.String("c", "", "config path")
	flag.Parse()

	// Boot configuration
	if err := config.Boot(configPath); err != nil {
		chk("config.Boot", err)
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

	players, artistGetters, err := loader.Load(ctx, version)
	chk("loader.Load", err)

	tgTeletrack := telegram.NewTeletrackMessenger(tgBot)
	teletrackd := core.New(players, artistGetters, tgTeletrack, tgTeletrack, slog.Default())

	go func() {
		err := teletrackd.Start(ctx)
		if err != nil {
			slog.Error("teletrackd.Start", "error", err.Error())
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
