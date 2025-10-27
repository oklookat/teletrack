package spotify

import (
	"context"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/oklookat/teletrack/config"
	"github.com/oklookat/teletrack/lastfm"
	"github.com/oklookat/teletrack/spoty"
	spotifyapi "github.com/zmb3/spotify/v2"
)

const (
	rateLimitSec     = 4
	rateLimit        = rateLimitSec * time.Second
	lastProgressIdle = 3 * (rateLimit / 2)
)

type Player struct {
	client   *spotifyapi.Client
	hooks    SpotifyPlayerHooks
	onError  func(error) error
	shutdown chan struct{}
	wg       sync.WaitGroup

	sync.RWMutex
	lastPlayed       *spoty.CurrentPlaying
	lastProgressTime time.Time
}

func NewPlayer(client *spotifyapi.Client, onError func(error) error) *Player {
	player := &Player{
		client:   client,
		onError:  onError,
		shutdown: make(chan struct{}),
	}
	player.hooks = newSpotifyPlayerHookImpl(lastfm.NewClient(config.C.LastFm.APIKey), onError, player.shutdown)
	return player
}

func (p *Player) Handle(ctx context.Context, b *bot.Bot) {
	p.wg.Add(1)
	go p.monitorLoop(ctx, b)
}

func (p *Player) Shutdown() {
	close(p.shutdown)
	p.wg.Wait()
}

func (p *Player) monitorLoop(ctx context.Context, b *bot.Bot) {
	defer p.wg.Done()
	ticker := time.NewTicker(rateLimit)
	defer ticker.Stop()

	for {
		select {
		case <-p.shutdown:
			return
		case <-ctx.Done():
			if p.onError != nil {
				p.onError(ctx.Err())
			}
			return
		case <-ticker.C:
			if err := p.handleTick(ctx, b); err != nil && p.onError != nil {
				p.onError(err)
			}
		}
	}
}

func (p *Player) handleTick(ctx context.Context, b *bot.Bot) error {
	currentPlaying, err := spoty.GetCurrentPlaying(ctx, p.client)
	if err != nil {
		return wrapErr("get current playing", err)
	}
	if p.hooks == nil {
		return nil
	}

	now := time.Now()
	p.Lock()
	defer p.Unlock()

	if currentPlaying == nil {
		p.hooks.OnNothingPlaying(ctx, b)
		p.lastPlayed = nil
		p.lastProgressTime = time.Time{}
		return nil
	}

	if p.lastPlayed != nil && currentPlaying.ID == p.lastPlayed.ID && !currentPlaying.Playing {
		if !p.lastProgressTime.IsZero() && now.Sub(p.lastProgressTime) > lastProgressIdle {
			p.hooks.OnNothingPlaying(ctx, b)
			return nil
		}
		p.hooks.OnOldTrackStillPlaying(ctx, b, currentPlaying)
		return nil
	}

	p.lastProgressTime = now
	if p.lastPlayed == nil || p.lastPlayed.ID != currentPlaying.ID {
		p.hooks.OnNewTrackPlayed(ctx, b, currentPlaying)
	} else {
		p.hooks.OnOldTrackStillPlaying(ctx, b, currentPlaying)
	}
	p.lastPlayed = currentPlaying
	return nil
}
