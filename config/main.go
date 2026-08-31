// Package config loads and persists teletrack configuration from a JSON file
// and environment variables.
//
// Precedence (highest wins): environment variables > config file > defaults.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/oklookat/teletrack/cache"
	"github.com/oklookat/teletrack/lastfm"
	"github.com/oklookat/teletrack/listenbrainz"
	"github.com/oklookat/teletrack/renderer/api"
	"github.com/oklookat/teletrack/renderer/html"
	"github.com/oklookat/teletrack/renderer/telegram"
	"github.com/oklookat/teletrack/spotify"
)

// Precedence: env vars > config.json file > built-in defaults on C.
const (
	_defaultConfigFileName = "config.json"
	_appName               = "teletrack"
	_envPrefix             = "TELETRACK"
)

type Service string

const (
	ServiceSpotify      Service = "spotify"
	ServiceLastFm       Service = "lastFm"
	ServiceListenBrainz Service = "listenBrainz"

	// Output renderers (status destinations).
	ServiceTelegram Service = "telegram"
	ServiceHTML     Service = "html"
	ServiceAPI      Service = "api"
)

// Env variables not related to Config,
// but related to other settings
const (
	// Path to directory with db, and other.
	EnvTeletrackData string = "TELETRACK_DATA"
)

// C holds the global, process-wide configuration instance.
//
// It is pre-populated with sane defaults so that a config file / env
// overrides always have a non-nil struct to land on.
var C = &Config{
	Schema:       "https://github.com/oklookat/teletrack/raw/refs/heads/main/config.schema.json",
	Telegram:     &telegram.Config{},
	HTML:         &html.Config{Addr: "127.0.0.1:8787"},
	API:          api.DefaultConfig(),
	LastFm:       &lastfm.Config{},
	Spotify:      &spotify.Config{},
	ListenBrainz: &listenbrainz.Config{},
	Players:      []Service{},
	Bios:         []Service{},
	Renderers:    []Service{ServiceTelegram},
	Cache:        cache.DefaultConfig(),
}

type Config struct {
	Schema string `json:"$schema"`

	// Output backends (used when listed in Renderers).
	Telegram *telegram.Config `json:"telegram"`
	HTML     *html.Config     `json:"html,omitempty"`
	API      *api.Config      `json:"api,omitempty"`

	// Modules (players / bios).
	Spotify      *spotify.Config      `json:"spotify"`      // player
	LastFm       *lastfm.Config       `json:"lastFm"`       // player + bio
	ListenBrainz *listenbrainz.Config `json:"listenBrainz"` // player + bio

	// Data sources.
	Players []Service `json:"players"`
	Bios    []Service `json:"bios"`

	// Renderers are status destinations, e.g. ["telegram", "html", "api"].
	// Updates are pushed to all enabled renderers in parallel.
	// "api" exposes a standalone HTTP API without a built-in UI.
	Renderers []Service `json:"renderers"`

	Cache *cache.Config `json:"cache,omitempty"`

	// path is the file this config was loaded from (or will be written to).
	path string `json:"-"`
}

// Boot loads configuration in the following order and returns the loaded
// instance (also stored in the package-level C for compatibility):
//
//  1. Look for config.json in, in order: current directory, $HOME/.teletrack/,
//     /etc/teletrack/. The first one found is loaded.
//  2. If no config file exists anywhere, write the current (default)
//     values of C to ./config.json.
//  3. Apply environment variable overrides on top of whatever was loaded.
//     Env vars always win, regardless of whether a config file was found.
//
// Callers should prefer the returned *Config over reading C directly.
func Boot(configPath *string) (*Config, error) {
	var (
		path  string
		found bool
	)

	if configPath != nil && *configPath != "" {
		path = *configPath
		found = true
	} else {
		path, found = findConfigFile()
	}

	if found {
		if err := loadConfigFile(path); err != nil {
			return nil, fmt.Errorf("load config file %q: %w", path, err)
		}
	} else {
		C.path = _defaultConfigFileName
		if err := C.Save(); err != nil {
			return nil, fmt.Errorf("write default config: %w", err)
		}
	}

	if err := applyEnvOverrides(_envPrefix, reflect.ValueOf(C)); err != nil {
		return nil, fmt.Errorf("apply env overrides: %w", err)
	}

	return C, nil
}

// Save writes the current configuration to disk as indented JSON.
func (c *Config) Save() error {
	path := c.path
	if path == "" {
		path = _defaultConfigFileName
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config file %q: %w", path, err)
	}

	return nil
}

// configSearchPaths returns candidate config.json locations, in order of
// precedence (first match wins).
func configSearchPaths() []string {
	paths := []string{filepath.Join(".", _defaultConfigFileName)}

	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, "."+_appName, _defaultConfigFileName))
	}

	paths = append(paths, filepath.Join("/etc", _appName, _defaultConfigFileName))

	return paths
}

// findConfigFile returns the first existing, regular config file among
// configSearchPaths.
func findConfigFile() (path string, found bool) {
	for _, p := range configSearchPaths() {
		info, err := os.Stat(p)
		if err == nil && !info.IsDir() {
			return p, true
		}
	}
	return "", false
}

// loadConfigFile reads and unmarshals path into C, then records the path
// on C so subsequent Save() calls target the same file.
func loadConfigFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, C); err != nil {
		return err
	}

	C.path = path

	return nil
}

// applyEnvOverrides walks value using reflection and, for every leaf
// field, checks whether a correspondingly-named environment variable is
// set. If so, it parses the raw string and assigns it, overriding
// whatever was already there.
//
// JSON tags are used to build env var names, since they already describe
// the shape people configure in config.json.
//
// Example:
//
//	ChatID string `json:"chatID"`
//
// nested under Telegram becomes env var:
//
//	TELETRACK_TELEGRAM_CHATID
func applyEnvOverrides(prefix string, value reflect.Value) error {
	value = dereference(value)
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return nil
	}

	valueType := value.Type()

	for i := 0; i < value.NumField(); i++ {
		structField := valueType.Field(i)
		fieldValue := value.Field(i)

		// Ignore unexported fields.
		if structField.PkgPath != "" {
			continue
		}

		jsonName, jsonSkip := parseJSONTag(structField.Tag.Get("json"))
		if jsonSkip {
			continue
		}
		if jsonName == "" {
			jsonName = structField.Name
		}

		envKey := prefix + "_" + strings.ToUpper(jsonName)

		// Recurse into nested structs (but not into time.Time, which is a
		// struct we want to treat as a leaf).
		if underlying := dereference(fieldValue); underlying.IsValid() &&
			underlying.Kind() == reflect.Struct &&
			underlying.Type() != reflect.TypeOf(time.Time{}) {
			if err := applyEnvOverrides(envKey, fieldValue); err != nil {
				return err
			}
			continue
		}

		raw, ok := os.LookupEnv(envKey)
		if !ok {
			continue
		}

		if err := setFieldFromString(fieldValue, raw); err != nil {
			return fmt.Errorf("env %s: %w", envKey, err)
		}
	}

	return nil
}

// setFieldFromString parses raw according to field's (possibly pointer)
// underlying kind and assigns it. field must be addressable/settable.
func setFieldFromString(field reflect.Value, raw string) error {
	if field.Kind() == reflect.Pointer {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}

	if field.Type() == reflect.TypeOf(time.Time{}) {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return err
		}
		field.Set(reflect.ValueOf(t))
		return nil
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)

	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		field.SetBool(b)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(n)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(n)

	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return err
		}
		field.SetFloat(f)

	case reflect.Slice:
		return setSliceFromString(field, raw)

	default:
		return fmt.Errorf("unsupported field kind %s for env override", field.Kind())
	}

	return nil
}

// setSliceFromString parses raw as a comma-separated list and assigns it
// to field, converting each element to the slice's element kind.
//
// Example:
//
//	Admins []int64 `json:"admins"`
//
// with TELETRACK_TELEGRAM_ADMINS=123,456,789 becomes []int64{123, 456, 789}.
//
// Whitespace around each element is trimmed. An empty raw string produces
// an empty (non-nil) slice, letting an env var explicitly clear a slice
// set in the config file.
func setSliceFromString(field reflect.Value, raw string) error {
	elemType := field.Type().Elem()

	if raw == "" {
		field.Set(reflect.MakeSlice(field.Type(), 0, 0))
		return nil
	}

	parts := strings.Split(raw, ",")
	result := reflect.MakeSlice(field.Type(), 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)

		elem := reflect.New(elemType).Elem()
		if err := setFieldFromString(elem, part); err != nil {
			return fmt.Errorf("element %q: %w", part, err)
		}

		result = reflect.Append(result, elem)
	}

	field.Set(result)

	return nil
}

// dereference follows pointer chains down to the underlying value.
// It returns a zero Value if a nil pointer is encountered along the way.
func dereference(value reflect.Value) reflect.Value {
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}

// parseJSONTag extracts the name portion of a `json:"..."` tag. A tag of
// "-" signals skip.
func parseJSONTag(tag string) (name string, skip bool) {
	if tag == "-" {
		return "", true
	}
	if tag == "" {
		return "", false
	}
	parts := strings.Split(tag, ",")
	return parts[0], false
}
