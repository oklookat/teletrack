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

var clientFactories = map[config.Service]func(context.Context, *config.Config) (any, error){
	config.ServiceSpotify: func(ctx context.Context, c *config.Config) (any, error) {
		return spotify.New(ctx, c.Spotify, spotifySaveToken)
	},
	config.ServiceLastFm: func(ctx context.Context, c *config.Config) (any, error) {
		return lastfm.NewClient(c.LastFm)
	},
	config.ServiceListenBrainz: func(ctx context.Context, c *config.Config) (any, error) {
		return listenbrainz.NewClient(c.ListenBrainz)
	},
}

func Load(ctx context.Context) ([]core.Player, []core.ArtistGetter, error) {
	players, err := loadServices[core.Player](ctx, config.C.Players, "player")
	if err != nil {
		return nil, nil, err
	}

	artistGetters, err := loadServices[core.ArtistGetter](ctx, config.C.Bios, "bio")
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

func loadServices[T any](ctx context.Context, services []config.Service, role string) ([]T, error) {
	enabled := shared.Unique(services)
	result := make([]T, 0, len(enabled))

	for _, name := range enabled {
		factory, ok := clientFactories[name]
		if !ok {
			slog.Warn("service not found in clientFactories", "service", name)
			continue
		}

		client, err := factory(ctx, config.C)
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

func spotifySaveToken(t *spotify.Token) error {
	config.C.Spotify.Authorize = false
	config.C.Spotify.Token = t
	return config.C.Save()
}
