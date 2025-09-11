package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"golang.org/x/oauth2"
)

const _configPath = "config.json"

var C = &Config{
	Telegram: &Telegram{},
	LastFm:   &LastFm{},
	Spotify:  &Spotify{},
}

type (
	LastFm struct {
		APIKey   string `json:"apiKey"`
		Username string `json:"username"`
	}

	Spotify struct {
		Authorize    bool          `json:"authorize"`
		RedirectURI  string        `json:"redirectURI"`
		ClientID     string        `json:"clientID"`
		ClientSecret string        `json:"clientSecret"`
		Token        *oauth2.Token `json:"token"`
	}

	Telegram struct {
		Token         string `json:"token"`
		UserID        int64  `json:"userID"`
		ChatID        string `json:"chatID"`
		ServiceChatID string `json:"serviceChatID"`
		MessageID     int    `json:"messageID"`
	}
)

type Config struct {
	Telegram *Telegram `json:"telegram"`
	LastFm   *LastFm   `json:"lastFm"`
	Spotify  *Spotify  `json:"spotify"`
}

// Save writes the config to the JSON file
func (c *Config) Save() error {
	f, err := os.OpenFile(_configPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
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
func Boot() error {
	f, err := os.Open(_configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Create new config file
			f, err := os.Create(_configPath)
			if err != nil {
				return fmt.Errorf("failed to create new config file: %w", err)
			}
			f.Close()

			if err := C.Save(); err != nil {
				return fmt.Errorf("failed to save new config: %w", err)
			}

			return fmt.Errorf("config created at %s; fill it and restart the application", _configPath)
		}
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(C); err != nil {
		return fmt.Errorf("failed to decode config JSON: %w", err)
	}

	return nil
}
