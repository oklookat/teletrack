package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/oklookat/teletrack/core/lastfm"
	"github.com/oklookat/teletrack/core/spotify"
)

const (
	_rateLimit        = 4 * time.Second
	_lastProgressIdle = 6 * time.Second

	_watermarkLink = "https://github.com/oklookat/teletrack"
	_watermark     = "powered by oklookat/teletrack"
)

type Messenger interface {
	UpdatePlaying(context.Context, *PlayingMessage) error
	UpdateIdle(context.Context, string) error
}

type Teletrack struct {
	player *spotify.Player
	lastFm *lastfm.Client

	messenger Messenger
	reporter  ErrorReporter

	mu sync.RWMutex

	shutdown chan struct{}
	done     chan struct{}

	currentMessage *PlayingMessage

	lastTrackID      string
	lastProgressTime time.Time

	vibeEmoji   string
	idleMessage string

	pausedTicks int
}

func New(
	player *spotify.Player,
	lastFm *lastfm.Client,
	messenger Messenger,
	reporter ErrorReporter,
	idleMessage string,
) *Teletrack {
	return &Teletrack{
		player: player,
		lastFm: lastFm,

		messenger: messenger,
		reporter:  reporter,

		idleMessage: idleMessage,

		shutdown: make(chan struct{}),
		done:     make(chan struct{}),
	}
}

func (t *Teletrack) Start(ctx context.Context) error {
	defer close(t.done)

	ticker := time.NewTicker(_rateLimit)
	defer ticker.Stop()

	for {
		select {
		case <-t.shutdown:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := t.handleTick(ctx); err != nil {
				if t.reporter != nil {
					t.reporter.ReportError(ctx, err)
				}
			}
		}
	}
}

func (t *Teletrack) Stop() {
	select {
	case <-t.shutdown:
		return
	default:
		close(t.shutdown)
	}
	<-t.done
}

func (t *Teletrack) handleTick(
	ctx context.Context,
) error {
	playing, err := t.player.GetPlaying(ctx)

	if err != nil {
		return fmt.Errorf(
			"spotify GetPlaying: %w",
			err,
		)
	}

	if t.isIdle(playing) {
		return t.onNothingPlaying(ctx)
	}

	now := time.Now()

	t.mu.RLock()

	lastID := t.lastTrackID
	lastProgress := t.lastProgressTime

	t.mu.RUnlock()

	if playing.ID == lastID {
		if !lastProgress.IsZero() &&
			now.Sub(lastProgress) > _lastProgressIdle {
			return t.onNothingPlaying(ctx)
		}

		t.mu.Lock()
		t.lastProgressTime = now
		t.mu.Unlock()

		return t.onOldTrackStillPlaying(
			ctx,
			playing,
		)
	}

	t.mu.Lock()

	t.lastTrackID = playing.ID
	t.lastProgressTime = now

	t.mu.Unlock()

	return t.onNewTrackPlaying(
		ctx,
		playing,
	)
}

func (t *Teletrack) isIdle(track *spotify.Track) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if track == nil {
		t.pausedTicks = 0
		return true
	}

	if track.Playing {
		t.pausedTicks = 0
		return false
	}

	t.pausedTicks++
	return t.pausedTicks >= 4
}

func (t *Teletrack) onNothingPlaying(ctx context.Context) error {
	t.mu.Lock()

	t.currentMessage = nil
	t.lastTrackID = ""
	t.lastProgressTime = time.Time{}

	if t.vibeEmoji == "" {
		t.vibeEmoji = totalRandomEmoji()
	}

	t.mu.Unlock()

	return t.messenger.UpdateIdle(ctx, t.idleMessage)
}

func (t *Teletrack) onOldTrackStillPlaying(
	ctx context.Context,
	track *spotify.Track,
) error {
	t.mu.RLock()

	current := t.currentMessage

	if current == nil {
		t.mu.RUnlock()
		return nil
	}

	msg := *current

	if msg.TrackInfo != nil {

		trackCopy := *msg.TrackInfo

		trackCopy.ProgressMs =
			track.ProgressMs
		trackCopy.Playing = track.Playing

		msg.TrackInfo = &trackCopy
	}

	t.mu.RUnlock()

	msg.Time = time.Now()

	t.mu.Lock()

	t.currentMessage = &msg

	t.mu.Unlock()

	return t.messenger.UpdatePlaying(
		ctx,
		&msg,
	)
}

func (t *Teletrack) onNewTrackPlaying(
	ctx context.Context,
	track *spotify.Track,
) error {
	t.mu.Lock()

	t.vibeEmoji =
		totalRandomEmoji()

	emoji := t.vibeEmoji

	t.mu.Unlock()

	bio, err := t.fetchArtistBio(
		ctx,
		track.Artist,
	)

	if err != nil {
		return err
	}

	msg := &PlayingMessage{
		ArtistInfo: bio,
		TrackInfo:  track,

		Time:  time.Now(),
		Emoji: emoji,

		Watermark:     _watermark,
		WatermarkLink: _watermarkLink,
	}

	t.mu.Lock()
	t.currentMessage = msg
	t.mu.Unlock()

	return t.messenger.UpdatePlaying(
		ctx,
		msg,
	)
}

func (t *Teletrack) fetchArtistBio(
	ctx context.Context,
	artist string,
) (*lastfm.ArtistBio, error) {

	return t.lastFm.GetArtistBio(
		ctx,
		artist,
		[]string{"en", "ru"},
	)
}
