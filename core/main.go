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
	_rateLimit = 4 * time.Second

	// How many consecutive "paused" ticks before we consider the player idle.
	_pausedTicksThreshold = 4

	// How long the reported progress can stay unchanged while Playing=true
	// before we treat the track as stalled/idle.
	_lastProgressIdle = 6 * time.Second
)

type Messenger interface {
	UpdatePlaying(context.Context, *PlayingMessage) error
	UpdateIdle(context.Context, *PlayingMessage) error
}

type Teletrack struct {
	player *spotify.Player
	lastFm *lastfm.Client

	messenger Messenger
	reporter  ErrorReporter

	mu sync.RWMutex

	shutdown chan struct{}
	done     chan struct{}

	currentMessage PlayingMessage

	lastTrackID      string
	lastProgressMs   int
	lastProgressTime time.Time

	pausedTicks int

	// wasIdle remembers whether the previous tick ended in the idle state.
	// Used to detect the idle -> playing transition so stale timing state
	// (accumulated while paused) doesn't leak into the "still playing" checks.
	wasIdle bool
}

func New(
	player *spotify.Player,
	lastFm *lastfm.Client,
	messenger Messenger,
	reporter ErrorReporter,
) *Teletrack {
	return &Teletrack{
		player: player,
		lastFm: lastFm,

		messenger:      messenger,
		reporter:       reporter,
		currentMessage: newPlayingMessage(nil, nil),

		shutdown: make(chan struct{}),
		done:     make(chan struct{}),

		// Start "idle" so the very first tick, if it's already playing,
		// is treated as a fresh start rather than a stale resume.
		wasIdle: true,
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

func (t *Teletrack) handleTick(ctx context.Context) error {
	playing, err := t.player.GetPlaying(ctx)
	if err != nil {
		return fmt.Errorf("spotify GetPlaying: %w", err)
	}

	idle := t.isIdle(playing)

	t.mu.Lock()
	wasIdle := t.wasIdle
	t.wasIdle = idle
	t.mu.Unlock()

	if idle {
		return t.onNothingPlaying(ctx)
	}

	now := time.Now()

	// We just came out of idle (unpaused, or player just became active
	// again). Any timing/progress state we were holding onto predates
	// the idle period and must not be used to judge staleness now.
	if wasIdle {
		t.mu.Lock()
		t.lastProgressTime = now
		t.lastProgressMs = playing.ProgressMs
		t.mu.Unlock()

		if playing.ID != t.getLastTrackID() {
			t.mu.Lock()
			t.lastTrackID = playing.ID
			t.mu.Unlock()
			return t.onNewTrackPlaying(ctx, playing)
		}

		return t.onOldTrackStillPlaying(ctx, playing)
	}

	t.mu.RLock()
	lastID := t.lastTrackID
	lastProgressMs := t.lastProgressMs
	lastProgressTime := t.lastProgressTime
	t.mu.RUnlock()

	if playing.ID == lastID {
		progressMoved := playing.ProgressMs != lastProgressMs

		if progressMoved {
			t.mu.Lock()
			t.lastProgressMs = playing.ProgressMs
			t.lastProgressTime = now
			t.mu.Unlock()
		} else if !lastProgressTime.IsZero() &&
			now.Sub(lastProgressTime) > _lastProgressIdle {
			// Playing=true but progress hasn't moved for too long: treat as idle.
			return t.onNothingPlaying(ctx)
		}

		return t.onOldTrackStillPlaying(ctx, playing)
	}

	t.mu.Lock()
	t.lastTrackID = playing.ID
	t.lastProgressMs = playing.ProgressMs
	t.lastProgressTime = now
	t.mu.Unlock()

	return t.onNewTrackPlaying(ctx, playing)
}

func (t *Teletrack) getLastTrackID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastTrackID
}

// isIdle reports whether the player should currently be treated as idle.
// It debounces short pauses: playback must be reported as paused for
// _pausedTicksThreshold consecutive ticks before we call it idle.
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
	return t.pausedTicks >= _pausedTicksThreshold
}

func (t *Teletrack) onNothingPlaying(ctx context.Context) error {
	return t.messenger.UpdateIdle(ctx, &t.currentMessage)
}

func (t *Teletrack) onOldTrackStillPlaying(
	ctx context.Context,
	track *spotify.Track,
) error {
	t.mu.Lock()

	if t.currentMessage.TrackInfo != nil {
		trackCopy := *t.currentMessage.TrackInfo

		trackCopy.ProgressMs = track.ProgressMs
		trackCopy.Playing = track.Playing

		t.currentMessage.TrackInfo = &trackCopy
	}

	t.currentMessage.Time = time.Now()

	t.mu.Unlock()

	return t.messenger.UpdatePlaying(ctx, &t.currentMessage)
}

func (t *Teletrack) onNewTrackPlaying(
	ctx context.Context,
	track *spotify.Track,
) error {
	if track == nil || track.ID == "" {
		return t.messenger.UpdatePlaying(ctx, &t.currentMessage)
	}

	bio, err := t.fetchArtistBio(ctx, track.Artist)
	if err != nil {
		return err
	}

	t.mu.Lock()
	t.currentMessage = newPlayingMessage(bio, track)
	t.mu.Unlock()

	return t.messenger.UpdatePlaying(ctx, &t.currentMessage)
}

func (t *Teletrack) fetchArtistBio(
	ctx context.Context,
	artist string,
) (*lastfm.ArtistBio, error) {
	return t.lastFm.GetArtistBio(ctx, artist, []string{"en", "ru"})
}
