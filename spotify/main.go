// Package spotify implements a core.Player backed by the Spotify Web API
// (currently playing endpoint) and OAuth2 authorization.
package spotify

import (
	"context"
	"fmt"

	"github.com/oklookat/teletrack/core"
	"github.com/oklookat/teletrack/shared"
	spotifyapi "github.com/zmb3/spotify/v2"
)

func New(ctx context.Context, cfg *Config, saveToken func(*Token) error) (*Player, error) {
	auth := newAuthorizer(cfg)
	token, err := auth.Authorize(ctx)
	if err != nil {
		return nil, err
	}
	if token != nil {
		if err := saveToken(token); err != nil {
			return nil, err
		}
	}

	client := auth.getClient(cfg.RedirectURI, cfg.ClientID, cfg.ClientSecret, cfg.Token, saveToken)

	return &Player{client: client}, nil
}

type Player struct {
	client *spotifyapi.Client
}

func (p *Player) GetPlaying(ctx context.Context) (core.Track, error) {
	curPlay, err := getCurrentPlaying(ctx, p.client)
	if err != nil {
		return nil, fmt.Errorf("getCurrentPlaying: %w", err)
	}
	if curPlay == nil {
		return nil, nil
	}

	track := &TrackInfo{
		track:      curPlay.Name,
		artist:     curPlay.Artist,
		spotifyId:  curPlay.ID,
		progressMs: shared.Ptr(curPlay.ProgressMs),
		durationMs: shared.Ptr(curPlay.DurationMs),
		playing:    curPlay.Playing,
	}

	if curPlay.CoverURL != nil && *curPlay.CoverURL != "" {
		track.coverURL = *curPlay.CoverURL
	}

	return track, nil
}
