package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oklookat/teletrack/cache"
	"github.com/oklookat/teletrack/shared"
	"golang.org/x/sync/singleflight"
)

const (
	rateLimit            = 4 * time.Second
	pausedTicksThreshold = 4
	lastProgressIdle     = 6 * time.Second
	artistFetchTimeout   = 10 * time.Second

	artistCachePrefix = "artist:"
)

type playbackState struct {
	lastTrackID      string
	lastProgressMs   int
	lastProgressTime time.Time

	pausedTicks int
	wasIdle     bool
}

type Teletrack struct {
	players       []Player
	artistGetters []ArtistGetter
	cache         cache.Cache
	sfGroup       singleflight.Group

	messenger Messenger
	reporter  ErrorReporter
	logger    *slog.Logger

	mu       sync.RWMutex
	wg       sync.WaitGroup
	stopOnce sync.Once
	shutdown chan struct{}
	done     chan struct{}

	currentMessage PlayingMessage
	playback       playbackState

	artistFetchID atomic.Uint64
}

func New(
	version string,
	players []Player,
	artistGetters []ArtistGetter,
	c cache.Cache,
	messenger Messenger,
	reporter ErrorReporter,
	logger *slog.Logger,
) (*Teletrack, error) {
	if c == nil {
		return nil, errors.New("cache instance is required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &Teletrack{
		players:        players,
		artistGetters:  artistGetters,
		cache:          c,
		messenger:      messenger,
		reporter:       reporter,
		logger:         logger,
		currentMessage: newPlayingMessage(nil, nil),
		shutdown:       make(chan struct{}),
		done:           make(chan struct{}),
		playback: playbackState{
			wasIdle: true,
		},
	}, nil
}

func (t *Teletrack) Start(parentCtx context.Context) error {
	defer close(t.done)

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	t.logger.Info("starting teletrack core loop")

	ticker := time.NewTicker(rateLimit)
	defer ticker.Stop()

	for {
		select {
		case <-t.shutdown:
			t.logger.Info("teletrack core loop stopped via shutdown signal")
			return nil

		case <-ctx.Done():
			t.logger.Info("teletrack core loop stopped via context cancellation")
			return ctx.Err()

		case <-ticker.C:
			if err := t.handleTick(ctx); err != nil {
				t.reportError(ctx, "handleTick failed", err)
			}
		}
	}
}

func (t *Teletrack) Stop() {
	t.stopOnce.Do(func() {
		close(t.shutdown)
		<-t.done
		t.wg.Wait()

		if t.cache != nil {
			_ = t.cache.Close()
		}
	})
}

func (t *Teletrack) handleTick(ctx context.Context) error {
	track, err := t.getPlaying(ctx)
	if err != nil {
		return err
	}

	idle, wasIdle := t.updatePlaybackState(track)

	if idle {
		return t.onNothingPlaying(ctx)
	}

	if wasIdle {
		return t.handleResume(ctx, track)
	}

	return t.handlePlaying(ctx, track)
}

func (t *Teletrack) getPlaying(ctx context.Context) (TrackInfoer, error) {
	for i, player := range t.players {
		track, err := player.GetPlaying(ctx)
		if err != nil {
			t.reportError(ctx, "player failed to fetch playing track", err, slog.Int("player_index", i))
			continue
		}

		if track == nil {
			continue
		}

		if track.ID() == "" {
			t.reportError(
				ctx,
				"player returned a track with empty ID",
				fmt.Errorf("empty track ID from player %d", i),
				slog.Int("player_index", i),
			)
			continue
		}

		return track, nil
	}

	return nil, nil
}

func (t *Teletrack) updatePlaybackState(track TrackInfoer) (idle bool, wasIdle bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	wasIdle = t.playback.wasIdle

	if track == nil {
		t.playback.pausedTicks = 0
		t.playback.wasIdle = true
		return true, wasIdle
	}

	if track.Playing() {
		t.playback.pausedTicks = 0
		t.playback.wasIdle = false
		return false, wasIdle
	}

	t.playback.pausedTicks++

	if t.playback.pausedTicks >= pausedTicksThreshold {
		t.playback.wasIdle = true
		return true, wasIdle
	}

	return false, wasIdle
}

func (t *Teletrack) handleResume(ctx context.Context, track TrackInfoer) error {
	now := time.Now()

	t.mu.Lock()
	lastTrackID := t.playback.lastTrackID
	t.playback.lastTrackID = track.ID()
	t.playback.lastProgressMs = progressMs(track)
	t.playback.lastProgressTime = now
	t.mu.Unlock()

	t.logger.Debug("playback resumed", slog.String("track_id", track.ID()))

	if track.ID() != lastTrackID {
		return t.onNewTrackPlaying(ctx, track)
	}

	return t.onOldTrackStillPlaying(ctx, track)
}

func (t *Teletrack) handlePlaying(ctx context.Context, track TrackInfoer) error {
	now := time.Now()

	t.mu.RLock()
	lastTrackID := t.playback.lastTrackID
	lastProgressMs := t.playback.lastProgressMs
	lastProgressTime := t.playback.lastProgressTime
	t.mu.RUnlock()

	if track.ID() != lastTrackID {
		return t.handleTrackChange(ctx, track, now)
	}

	if !supportsProgress(track) {
		return t.onOldTrackStillPlaying(ctx, track)
	}

	currentProgress := *track.ProgressMs()

	if currentProgress != lastProgressMs {
		t.mu.Lock()
		t.playback.lastProgressMs = currentProgress
		t.playback.lastProgressTime = now
		t.mu.Unlock()

		return t.onOldTrackStillPlaying(ctx, track)
	}

	if !lastProgressTime.IsZero() && now.Sub(lastProgressTime) > lastProgressIdle {
		t.logger.Debug("playback stalled (progress unchanged for too long)",
			slog.String("track_id", track.ID()),
			slog.Duration("idle_duration", now.Sub(lastProgressTime)),
		)
		return t.onNothingPlaying(ctx)
	}

	return t.onOldTrackStillPlaying(ctx, track)
}

func (t *Teletrack) handleTrackChange(ctx context.Context, track TrackInfoer, now time.Time) error {
	t.mu.Lock()
	t.playback.lastTrackID = track.ID()
	t.playback.lastProgressMs = progressMs(track)
	t.playback.lastProgressTime = now
	t.mu.Unlock()

	return t.onNewTrackPlaying(ctx, track)
}

func (t *Teletrack) onNothingPlaying(ctx context.Context) error {
	t.mu.RLock()
	message := t.currentMessage
	t.mu.RUnlock()

	return t.messenger.UpdateIdle(ctx, &message)
}

func (t *Teletrack) onOldTrackStillPlaying(ctx context.Context, track TrackInfoer) error {
	t.mu.Lock()
	t.currentMessage.TrackInfo = track
	t.currentMessage.Time = trackTime(track)
	message := t.currentMessage
	t.mu.Unlock()

	return t.messenger.UpdatePlaying(ctx, &message)
}

func (t *Teletrack) onNewTrackPlaying(ctx context.Context, track TrackInfoer) error {
	artist := track.Artist()
	cacheKey := artistCachePrefix + artist

	if cachedData, ok := t.cache.Get(ctx, cacheKey); ok {
		if bio, err := bytesToArtistInfo(cachedData); err == nil {
			t.logger.DebugContext(ctx, "cache hit for artist bio", slog.String("artist", artist), slog.String("cache_key", cacheKey))
			message := newPlayingMessage(bio, track)

			t.mu.Lock()
			t.currentMessage = message
			t.mu.Unlock()

			t.artistFetchID.Add(1)
			return t.messenger.UpdatePlaying(ctx, &message)
		}
		t.logger.WarnContext(ctx, "corrupted cache entry for artist bio", slog.String("artist", artist), slog.String("cache_key", cacheKey))
	} else {
		t.logger.DebugContext(ctx, "cache miss for artist bio", slog.String("artist", artist), slog.String("cache_key", cacheKey))
	}

	message := newPlayingMessage(dummyArtistInfo{}, track)
	fetchID := t.artistFetchID.Add(1)

	t.mu.Lock()
	t.currentMessage = message
	trackID := track.ID()
	t.mu.Unlock()

	if err := t.messenger.UpdatePlaying(ctx, &message); err != nil {
		return err
	}

	t.wg.Add(1)
	go t.fetchArtistInfoAsync(ctx, artist, trackID, fetchID)

	return nil
}

func (t *Teletrack) fetchArtistInfoAsync(parentCtx context.Context, artist, trackID string, fetchID uint64) {
	defer t.wg.Done()

	fetchCtx, cancel := context.WithTimeout(parentCtx, artistFetchTimeout)
	defer cancel()

	bio, found := t.fetchArtistInfo(fetchCtx, artist)
	if !found {
		bio = dummyArtistInfo{}
	}

	if t.artistFetchID.Load() != fetchID {
		t.logger.Debug("discarding outdated artist bio", slog.String("artist", artist), slog.Uint64("fetch_id", fetchID))
		return
	}

	t.mu.Lock()
	if t.currentMessage.TrackInfo == nil || t.currentMessage.TrackInfo.ID() != trackID {
		t.mu.Unlock()
		return
	}
	t.currentMessage.ArtistInfo = bio
	updatedMessage := t.currentMessage
	t.mu.Unlock()

	if err := t.messenger.UpdatePlaying(parentCtx, &updatedMessage); err != nil {
		t.reportError(parentCtx, "failed to update playing message with async artist bio", err)
	}
}

func (t *Teletrack) fetchArtistInfo(ctx context.Context, artist string) (ArtistInfoer, bool) {
	cacheKey := artistCachePrefix + artist

	if t.cache.IsFailed(ctx, cacheKey) {
		t.logger.DebugContext(ctx, "negative cache hit: skipping artist lookup", slog.String("artist", artist), slog.String("cache_key", cacheKey))
		return nil, false
	}

	val, err, _ := t.sfGroup.Do(artist, func() (any, error) {
		t.logger.DebugContext(ctx, "fetching artist info from remote getters", slog.String("artist", artist))

		for i, getter := range t.artistGetters {
			artistInfo, err := getter.GetArtistInfo(ctx, artist, []string{"en", "ru"})
			if err != nil {
				t.reportError(ctx, "artist getter failed", err, slog.Int("getter_index", i), slog.String("artist", artist))
				continue
			}

			if artistInfo.Bio() == "" || artistInfo.BioService() == "" || artistInfo.Link() == "" {
				continue
			}

			if bytesData, err := artistInfoToBytes(artistInfo); err == nil {
				_ = t.cache.Set(ctx, cacheKey, bytesData, 0)
				t.logger.DebugContext(ctx, "cached new artist bio", slog.String("artist", artist), slog.String("cache_key", cacheKey))
			}
			return artistInfo, nil
		}

		_ = t.cache.SetFailed(ctx, cacheKey, 0)
		t.logger.DebugContext(ctx, "cached negative result for artist bio", slog.String("artist", artist), slog.String("cache_key", cacheKey))
		return nil, fmt.Errorf("artist info not found")
	})

	if err != nil {
		return nil, false
	}

	info, ok := val.(ArtistInfoer)
	return info, ok
}

func (t *Teletrack) reportError(ctx context.Context, msg string, err error, attrs ...slog.Attr) {
	args := make([]any, 0, len(attrs)+1)
	args = append(args, slog.Any("error", err))
	for _, attr := range attrs {
		args = append(args, attr)
	}

	t.logger.Error(msg, args...)

	if t.reporter != nil {
		t.reporter.ReportError(ctx, fmt.Errorf("%s: %w", msg, err))
	}
}

func supportsProgress(track TrackInfoer) bool {
	return shared.TrackProgressSupported(track.ProgressMs(), track.DurationMs())
}

func progressMs(track TrackInfoer) int {
	if !supportsProgress(track) {
		return 0
	}
	return *track.ProgressMs()
}

func trackTime(track TrackInfoer) time.Time {
	if track.Time() != nil {
		return *track.Time()
	}
	return time.Now()
}
