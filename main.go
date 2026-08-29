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
	"github.com/oklookat/teletrack/telegram"
)

var version = "1.0.0-debug"

func main() {
	if err := run(); err != nil {
		slog.Error("application exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	slog.Info("starting teletrack", "version", version)

	// Flags
	configPath := flag.String("c", "", "path to configuration file")
	statePath := flag.String("D", "./data", "state and data storage directory")
	flag.Parse()

	dataDir, err := prepareDataDir(*statePath)
	if err != nil {
		return fmt.Errorf("prepare data directory: %w", err)
	}

	// Boot configuration
	if err := config.Boot(configPath); err != nil {
		return fmt.Errorf("boot configuration: %w", err)
	}

	// Signal-aware context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Telegram bot initialization
	tgBot, err := telegram.NewTelegramBot(ctx, stop, version, config.C.Telegram)
	if err != nil {
		return fmt.Errorf("start telegram bot: %w", err)
	}

	players, artistGetters, err := loader.Load(ctx, version)
	if err != nil {
		return fmt.Errorf("load components: %w", err)
	}

	tgTeletrack := telegram.NewTeletrackMessenger(tgBot)

	// SQLite cache setup (filepath.Join handles cross-platform path slashes properly)
	dbPath := filepath.Join(dataDir, "cache.db")
	cacheDB, err := cache.NewSQLiteCache(dbPath, config.C.Cache, slog.Default())
	if err != nil {
		return fmt.Errorf("initialize sqlite cache: %w", err)
	}
	defer func() {
		if err := cacheDB.Close(); err != nil {
			slog.Error("failed to close cache database cleanly", "error", err)
		}
	}()

	teletrackd, err := core.New(version, players, artistGetters, cacheDB, tgTeletrack, tgTeletrack, slog.Default())
	if err != nil {
		return fmt.Errorf("create core daemon: %w", err)
	}

	// Run daemon asynchronously and catch non-cancellation errors
	errChan := make(chan error, 1)
	go func() {
		if err := teletrackd.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errChan <- err
		}
	}()

	// Wait for OS shutdown signals or a critical startup/runtime failure
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received, stopping daemon...")
	case err := <-errChan:
		return fmt.Errorf("teletrackd runtime error: %w", err)
	}

	teletrackd.Stop()
	slog.Info("teletrack stopped gracefully")
	return nil
}

func prepareDataDir(flagStatePath string) (string, error) {
	trueStatePath := "./"

	if envStatePath := os.Getenv("TELETRACK_DATA"); envStatePath != "" {
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
