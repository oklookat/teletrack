package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/oklookat/teletrack/cache"
	"github.com/oklookat/teletrack/config"
	"github.com/oklookat/teletrack/core"
	"github.com/oklookat/teletrack/loader"
	"github.com/oklookat/teletrack/renderer/telegram"
	"github.com/oklookat/teletrack/shared"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	slog.Info("starting teletrack", "version", shared.Version)

	// Flags
	configPath := flag.String("c", "", "path to configuration file")
	statePath := flag.String("D", "./data", "state and data storage directory")
	flag.Parse()

	dataDir, err := prepareDataDir(*statePath)
	if err != nil {
		return fmt.Errorf("prepare data directory: %w", err)
	}

	cfg, err := config.Boot(configPath)
	if err != nil {
		return fmt.Errorf("boot configuration: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	players, artistGetters, err := loader.Load(ctx, cfg)
	if err != nil {
		return fmt.Errorf("load components: %w", err)
	}

	var tgBot *telegram.TelegramBot
	if containsService(cfg.Renderers, config.ServiceTelegram) {
		tgBot, err = telegram.NewTelegramBot(ctx, stop, cfg.Telegram)
		if err != nil {
			return fmt.Errorf("start telegram bot: %w", err)
		}
	}

	renderers, err := loader.LoadRenderers(ctx, cfg, loader.RendererDeps{TelegramBot: tgBot})
	if err != nil {
		return fmt.Errorf("load renderers: %w", err)
	}

	dbPath := filepath.Join(dataDir, "cache.db")
	cacheDB, err := cache.NewSQLiteCache(dbPath, cfg.Cache, slog.Default())
	if err != nil {
		return fmt.Errorf("initialize sqlite cache: %w", err)
	}
	defer func() {
		if err := cacheDB.Close(); err != nil {
			slog.Error("failed to close cache database cleanly", "error", err)
		}
	}()

	tCore, err := core.New(players, artistGetters, cacheDB, renderers, slog.Default())
	if err != nil {
		return fmt.Errorf("create core: %w", err)
	}

	errCh := make(chan error, 1)

	go func() {
		if err := tCore.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received, stopping daemon...")
	case err := <-errCh:
		return fmt.Errorf("core runtime error: %w", err)
	}

	// Stop only the core loop and background artist fetches.
	// The SQLite cache is closed by the defer above — do not close it again
	// inside core.Stop (see core.Stop docs).
	tCore.Stop()
	slog.Info("core stopped gracefully")
	return nil
}

func prepareDataDir(flagStatePath string) (string, error) {
	trueStatePath := "./"

	if envStatePath := os.Getenv(config.EnvTeletrackData); envStatePath != "" {
		trueStatePath = envStatePath
	} else if flagStatePath != "" {
		trueStatePath = flagStatePath
	}

	trueStatePath = filepath.Clean(trueStatePath)
	if err := os.MkdirAll(trueStatePath, 0750); err != nil {
		return "", err
	}
	return trueStatePath, nil
}

func containsService(list []config.Service, want config.Service) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
