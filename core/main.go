package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	_rateLimit = 4 * time.Second

	// How many consecutive "paused" ticks before we consider the player idle.
	_pausedTicksThreshold = 4

	// How long the reported progress can stay unchanged while Playing=true
	// before we treat the track as stalled/idle.
	_lastProgressIdle = 6 * time.Second
)

type Player interface {
	// Get current playing track.
	GetPlaying(ctx context.Context) (*TrackInfo, error)
}

type Messenger interface {
	UpdatePlaying(context.Context, *PlayingMessage) error
	UpdateIdle(context.Context, *PlayingMessage) error
}

type Teletrack struct {
	players      []Player
	artistGetter ArtistGetter

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
	players []Player,
	artistGetter ArtistGetter,
	messenger Messenger,
	reporter ErrorReporter,
) *Teletrack {
	return &Teletrack{
		players:      players,
		artistGetter: artistGetter,

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
	var playing *TrackInfo

	for i, player := range t.players {
		tInfo, err := player.GetPlaying(ctx)
		if err != nil {
			if i == len(t.players)-1 {
				return err
			}
			t.reporter.ReportError(ctx, fmt.Errorf("handleTick.GetPlaying: %w", err))
			continue
		}
		playing = tInfo
		break
	}

	if playing != nil {
		if ok := playing.GenerateID(); !ok {
			return nil
		}
	}

	idle := t.isIdle(playing)

	t.mu.Lock()
	wasIdle := t.wasIdle
	t.wasIdle = idle
	t.mu.Unlock()

	if idle {
		return t.onNothingPlaying(ctx)
	}

	if t.currentMessage.TrackInfo == nil {
		return t.onNewTrackPlaying(ctx, playing)
	}

	now := time.Now()
	supportsProgress := playing.ProgressSupported()

	if wasIdle {
		t.mu.Lock()
		t.lastProgressTime = now
		if supportsProgress {
			t.lastProgressMs = *playing.ProgressMs
		} else {
			t.lastProgressMs = 0
		}
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
		// Если поставщик не отдаёт прогресс, у нас нет данных, чтобы
		// судить о "залипании" — доверяем playing=true как есть.
		if !supportsProgress {
			return t.onOldTrackStillPlaying(ctx, playing)
		}

		currentProgress := *playing.ProgressMs
		progressMoved := currentProgress != lastProgressMs

		if progressMoved {
			t.mu.Lock()
			t.lastProgressMs = currentProgress
			t.lastProgressTime = now
			t.mu.Unlock()
		} else if !lastProgressTime.IsZero() &&
			now.Sub(lastProgressTime) > _lastProgressIdle {
			// Playing=true, но прогресс не двигается слишком долго: считаем idle.
			return t.onNothingPlaying(ctx)
		}

		return t.onOldTrackStillPlaying(ctx, playing)
	}

	t.mu.Lock()
	t.lastTrackID = playing.ID
	if supportsProgress {
		t.lastProgressMs = *playing.ProgressMs
	} else {
		t.lastProgressMs = 0
	}
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
func (t *Teletrack) isIdle(track *TrackInfo) bool {
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

	if t.pausedTicks >= _pausedTicksThreshold {
		return true
	} else {
		t.pausedTicks++
		return false
	}
}

func (t *Teletrack) onNothingPlaying(ctx context.Context) error {
	return t.messenger.UpdateIdle(ctx, &t.currentMessage)
}

func (t *Teletrack) onOldTrackStillPlaying(
	ctx context.Context,
	track *TrackInfo,
) error {
	t.mu.Lock()

	if t.currentMessage.TrackInfo != nil {
		trackCopy := *t.currentMessage.TrackInfo

		trackCopy.ProgressMs = track.ProgressMs
		trackCopy.Playing = track.Playing

		t.currentMessage.TrackInfo = &trackCopy
	}

	if track != nil && track.Time == nil {
		t.currentMessage.Time = time.Now()
	} else {
		t.currentMessage.Time = *track.Time
	}

	t.mu.Unlock()

	return t.messenger.UpdatePlaying(ctx, &t.currentMessage)
}

func (t *Teletrack) onNewTrackPlaying(
	ctx context.Context,
	track *TrackInfo,
) error {
	// if track == nil || track.ID == "" {
	// 	return t.messenger.UpdatePlaying(ctx, &t.currentMessage)
	// }

	bio, err := t.fetchArtistInfo(ctx, track.Artist)
	if err != nil {
		return err
	}

	t.mu.Lock()
	t.currentMessage = newPlayingMessage(bio, track)
	t.mu.Unlock()

	return t.messenger.UpdatePlaying(ctx, &t.currentMessage)
}

type ArtistGetter interface {
	// First lang is preferred. Other langs for fallback if first lang doesnt have bio.

	// Langs format:

	// ISO639-2 code (see https://www.loc.gov/standards/iso639-2/php/code_list.php
	GetArtistInfo(ctx context.Context, artist string, langs []string) (*ArtistInfo, error)
}

type ArtistInfo struct {
	Link       string
	Bio        string
	BioService string
}

func (t *Teletrack) fetchArtistInfo(
	ctx context.Context,
	artist string,
) (*ArtistInfo, error) {
	return t.artistGetter.GetArtistInfo(ctx, artist, []string{"en", "ru"})
}
