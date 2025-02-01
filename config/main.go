package config

import (
	"encoding/json"
	"errors"
	"os"
)

const (
	_configPath = "config.json"
)

var (
	C = &Config{
		Telegram: &Telegram{},
		LastFm:   &LastFm{},
		Spotify:  &Spotify{},
	}
)

type Config struct {
	Telegram *Telegram `json:"telegram"`
	LastFm   *LastFm   `json:"lastFm"`
	Spotify  *Spotify  `json:"spotify"`
}

func (c *Config) Save() error {
	f, err := os.OpenFile(_configPath, os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "\t")
	err = enc.Encode(C)
	return err
}

func Boot() error {
	f, err := os.Open(_configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		} else {
			return err
		}
	}
	if f != nil {
		defer f.Close()
		return json.NewDecoder(f).Decode(C)
	}

	// New config. Create it.
	f, err = os.Create(_configPath)
	if err != nil {
		return err
	}
	f.Close()

	if err := C.Save(); err != nil {
		return err
	}

	println("Config created. Fill them, then start app.")
	os.Exit(0)

	return err
}
