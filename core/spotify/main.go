package spotify

import (
	"context"
	"fmt"
	"time"

	spotifyapi "github.com/zmb3/spotify/v2"
	"golang.org/x/oauth2"
)

const (
	rateLimitSec     = 4
	rateLimit        = rateLimitSec * time.Second
	lastProgressIdle = 3 * (rateLimit / 2)
)

type Config struct {
	Authorize    bool          `json:"authorize"`
	RedirectURI  string        `json:"redirectURI"`
	ClientID     string        `json:"clientID"`
	ClientSecret string        `json:"clientSecret"`
	Token        *oauth2.Token `json:"token"`
}

type Track struct {
	ID string
	// Playing now? (not paused?)
	Playing bool
	// "Blood Orange"
	Artist string
	// "Chewing Gum"
	Track string
	// Track link to Spotify.
	TrackLink string
	// Track cover URL. Can be emprty.
	CoverURL string
	// Track current progress in ms.
	ProgressMs int
	// Track total duration in ms.
	DurationMs int
}

func New(ctx context.Context, cfg *Config, saveToken func(*oauth2.Token) error) (*Player, error) {
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

	client := auth.getClient(cfg.RedirectURI, cfg.ClientID, cfg.ClientSecret, cfg.Token)

	player := &Player{
		client: client,
	}
	return player, nil
}

type Player struct {
	client *spotifyapi.Client
}

func (p *Player) GetPlaying(ctx context.Context) (*Track, error) {
	curPlay, err := getCurrentPlaying(ctx, p.client)
	if err != nil {
		return nil, fmt.Errorf("getCurrentPlaying: %w", err)
	}
	if curPlay == nil {
		return nil, nil
	}

	track := &Track{
		ID:         curPlay.ID,
		Track:      curPlay.Name,
		Artist:     curPlay.Artist,
		TrackLink:  "https://open.spotify.com/track/" + curPlay.ID,
		ProgressMs: curPlay.ProgressMs,
		DurationMs: curPlay.DurationMs,
		Playing:    curPlay.Playing,
	}
	if curPlay.CoverURL != nil && len(*curPlay.CoverURL) > 0 {
		track.CoverURL = *curPlay.CoverURL
	}

	return track, err
}
