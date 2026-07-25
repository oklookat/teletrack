package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/oklookat/teletrack/core/lastfm"
	"github.com/oklookat/teletrack/core/spotify"
	"github.com/oklookat/teletrack/telegram"
)

var C = &Config{
	Schema:   "https://github.com/oklookat/teletrack/raw/refs/heads/main/config.schema.json",
	Telegram: &telegram.Config{},
	LastFm:   &lastfm.Config{},
	Spotify:  &spotify.Config{},
}

type Config struct {
	Schema      string           `json:"$schema"`
	Telegram    *telegram.Config `json:"telegram"`
	LastFm      *lastfm.Config   `json:"lastFm"`
	Spotify     *spotify.Config  `json:"spotify"`
	IdleMessage string           `json:"idleMessage"`
	// internal
	path string
}

// Save writes the config to the JSON file
func (c *Config) Save() error {
	f, err := os.OpenFile(c.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open config file for saving: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "\t")
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("failed to encode config to JSON: %w", err)
	}
	return nil
}

// Boot loads the config from file or creates a new one if missing
func Boot(configPath string) error {
	f, err := os.Open(configPath)
	defer func() {
		if err == nil && C != nil {
			C.path = configPath
		}
	}()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Create new config file
			f, err := os.Create(configPath)
			if err != nil {
				return fmt.Errorf("failed to create new config file: %w", err)
			}
			f.Close()

			if err := C.Save(); err != nil {
				return fmt.Errorf("failed to save new config: %w", err)
			}

			return fmt.Errorf("config created at %s; fill it and restart the application", configPath)
		}
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(C); err != nil {
		return fmt.Errorf("failed to decode config JSON: %w", err)
	}

	return nil
}
