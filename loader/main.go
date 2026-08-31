// Package loader constructs Player and ArtistGetter implementations from
// the process configuration.
package loader

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/oklookat/teletrack/config"
	"github.com/oklookat/teletrack/core"
	"github.com/oklookat/teletrack/lastfm"
	"github.com/oklookat/teletrack/listenbrainz"
	"github.com/oklookat/teletrack/shared"
	"github.com/oklookat/teletrack/spotify"
)

// Load constructs players and artist getters from cfg.
// Spotify OAuth tokens are persisted via cfg.Save when authorization completes.
func Load(ctx context.Context, cfg *config.Config) ([]core.Player, []core.ArtistGetter, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("config is required")
	}

	saveToken := func(t *spotify.Token) error {
		cfg.Spotify.Authorize = false
		cfg.Spotify.Token = t
		return cfg.Save()
	}

	factories := map[config.Service]func(context.Context, *config.Config) (any, error){
		config.ServiceSpotify: func(ctx context.Context, c *config.Config) (any, error) {
			return spotify.New(ctx, c.Spotify, saveToken)
		},
		config.ServiceLastFm: func(_ context.Context, c *config.Config) (any, error) {
			return lastfm.NewClient(c.LastFm)
		},
		config.ServiceListenBrainz: func(_ context.Context, c *config.Config) (any, error) {
			return listenbrainz.NewClient(c.ListenBrainz)
		},
	}

	players, err := loadServices[core.Player](ctx, cfg, cfg.Players, factories, "player")
	if err != nil {
		return nil, nil, err
	}

	artistGetters, err := loadServices[core.ArtistGetter](ctx, cfg, cfg.Bios, factories, "bio")
	if err != nil {
		return nil, nil, err
	}

	if len(players) == 0 {
		slog.Warn("check your config: there are no players, there is nowhere to get the current track from")
	}
	if len(artistGetters) == 0 {
		slog.Warn("check your config: there are no bios, there is nowhere to get artist biographies from")
	}

	return players, artistGetters, nil
}

func loadServices[T any](
	ctx context.Context,
	cfg *config.Config,
	services []config.Service,
	factories map[config.Service]func(context.Context, *config.Config) (any, error),
	role string,
) ([]T, error) {
	enabled := shared.Unique(services)
	result := make([]T, 0, len(enabled))

	for _, name := range enabled {
		factory, ok := factories[name]
		if !ok {
			slog.Warn("service not found in clientFactories", "service", name)
			continue
		}

		client, err := factory(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("%s factory failed: %w", name, err)
		}

		if typedClient, ok := client.(T); ok {
			slog.Info("service loaded", "name", name, "role", role)
			result = append(result, typedClient)
		} else {
			slog.Error("service does not support functionality", "service", name, "functionality", role)
		}
	}

	return result, nil
}
