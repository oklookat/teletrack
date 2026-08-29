// Written by: Claude, Gemini, Grok
package core

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/oklookat/teletrack/shared"
)

const (
	_rateLimit = 4 * time.Second

	// Number of consecutive paused ticks required before playback
	// is considered idle.
	_pausedTicksThreshold = 4

	// Maximum amount of time progress may remain unchanged while
	// Playing=true before playback is considered stalled.
	_lastProgressIdle = 6 * time.Second

	// Timeout for background artist bio network requests.
	_artistFetchTimeout = 10 * time.Second

	// TTL for successfully resolved artist bios.
	_artistCacheTTL = 24 * time.Hour

	// TTL for failed lookups (dummy bio), kept short so we retry soon
	// instead of failing silently for a whole day.
	_artistCacheFailureTTL = 5 * time.Minute
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

	// Two separate caches so failed lookups don't sit in memory
	// alongside real bios with a mismatched TTL semantics.
	cachedArtistInfoer       *expirable.LRU[string, ArtistInfoer]
	cachedFailedArtistInfoer *expirable.LRU[string, struct{}]

	messenger Messenger
	reporter  ErrorReporter
	logger    *slog.Logger

	mu sync.RWMutex

	// Guards background goroutines (artist bio fetches) so Stop()
	// can wait for them to finish instead of leaking them.
	wg sync.WaitGroup

	shutdown chan struct{}
	done     chan struct{}

	currentMessage PlayingMessage
	playback       playbackState

	// Token used to invalidate stale async artist requests if the
	// track (or currentMessage) changes while a fetch is in flight.
	// Always guarded by mu; no atomics needed since every read/write
	// already happens under the lock.
	artistFetchID uint64
}

func New(
	players []Player,
	artistGetters []ArtistGetter,
	messenger Messenger,
	reporter ErrorReporter,
	logger *slog.Logger,
) *Teletrack {
	if logger == nil {
		logger = slog.Default()
	}

	return &Teletrack{
		players:                  players,
		artistGetters:            artistGetters,
		cachedArtistInfoer:       expirable.NewLRU[string, ArtistInfoer](100, nil, _artistCacheTTL),
		cachedFailedArtistInfoer: expirable.NewLRU[string, struct{}](100, nil, _artistCacheFailureTTL),
		messenger:                messenger,
		reporter:                 reporter,
		logger:                   logger,

		currentMessage: newPlayingMessage(nil, nil),

		shutdown: make(chan struct{}),
		done:     make(chan struct{}),

		playback: playbackState{
			wasIdle: true,
		},
	}
}

func (t *Teletrack) Start(ctx context.Context) error {
	defer close(t.done)

	t.logger.Info("starting teletrack core loop")

	ticker := time.NewTicker(_rateLimit)
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

// Stop signals the core loop to stop, waits for it to exit, then waits
// for any in-flight background work (e.g. artist bio fetches) to finish
// before returning. This makes it safe for callers to tear down shared
// resources (messenger connections, etc.) right after Stop returns.
func (t *Teletrack) Stop() {
	select {
	case <-t.shutdown:
		return
	default:
		close(t.shutdown)
	}

	<-t.done
	t.wg.Wait()
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
	// players are tried in priority order; the first one that returns
	// a valid track wins.
	for i, player := range t.players {
		track, err := player.GetPlaying(ctx)
		if err != nil {
			t.reportError(
				ctx,
				"player failed to fetch playing track",
				err,
				slog.Int("player_index", i),
			)
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

		t.logger.Debug("successfully retrieved active track",
			slog.Int("player_index", i),
			slog.String("track_id", track.ID()),
			slog.String("artist", track.Artist()),
			slog.String("title", track.Track()),
		)

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

	if t.playback.pausedTicks >= _pausedTicksThreshold {
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

	t.logger.Info("playback resumed", slog.String("track_id", track.ID()))

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

	if !lastProgressTime.IsZero() && now.Sub(lastProgressTime) > _lastProgressIdle {
		t.logger.Warn("playback stalled (progress unchanged for too long)",
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

	t.logger.Info("track changed",
		slog.String("track_id", track.ID()),
		slog.String("artist", track.Artist()),
		slog.String("title", track.Track()),
	)

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

// onNewTrackPlaying publishes the new track immediately (with a cached
// or placeholder bio) and, on cache miss, kicks off a background fetch
// for the artist bio. Every path that changes which track/message is
// "current" bumps artistFetchID, so a slow in-flight fetch for a
// previous track can never overwrite a newer message — this fixes the
// stale-bio race that existed when only the cache-miss path bumped it.
func (t *Teletrack) onNewTrackPlaying(ctx context.Context, track TrackInfoer) error {
	artist := track.Artist()

	// Check LRU cache synchronously first.
	if cachedBio, ok := t.cachedArtistInfoer.Get(artist); ok {
		t.logger.Debug("cache hit for artist bio", slog.String("artist", artist))
		message := newPlayingMessage(cachedBio, track)

		t.mu.Lock()
		t.currentMessage = message
		t.artistFetchID++ // invalidate any fetch still in flight for the previous track
		t.mu.Unlock()

		return t.messenger.UpdatePlaying(ctx, &message)
	}

	// Cache miss: publish track immediately with placeholder/dummy bio.
	message := newPlayingMessage(dummyArtistInfo{}, track)

	t.mu.Lock()
	t.currentMessage = message
	t.artistFetchID++
	fetchID := t.artistFetchID
	trackID := track.ID()
	t.mu.Unlock()

	if err := t.messenger.UpdatePlaying(ctx, &message); err != nil {
		return err
	}

	// Trigger async background bio lookup. Tracked via WaitGroup so
	// Stop() can wait for it, and uses a context derived from the
	// caller's ctx (not context.Background()) so it's cancelled
	// promptly on shutdown instead of running for up to
	// _artistFetchTimeout after Stop() has already returned.
	t.wg.Add(1)
	go t.fetchArtistInfoAsync(ctx, artist, trackID, fetchID)

	return nil
}

func (t *Teletrack) fetchArtistInfoAsync(parentCtx context.Context, artist, trackID string, fetchID uint64) {
	defer t.wg.Done()

	fetchCtx, cancel := context.WithTimeout(parentCtx, _artistFetchTimeout)
	defer cancel()

	bio, found := t.fetchArtistInfo(fetchCtx, artist)
	if !found {
		bio = dummyArtistInfo{}
	}

	t.mu.Lock()

	// Guard against race conditions: discard the fetched bio if a
	// newer message was published while this fetch was in flight
	// (covers both "another track started playing" and "a cache hit
	// for a different track happened").
	if t.artistFetchID != fetchID || t.currentMessage.TrackInfo == nil || t.currentMessage.TrackInfo.ID() != trackID {
		t.mu.Unlock()
		t.logger.Debug("discarding outdated artist bio",
			slog.String("artist", artist),
			slog.Uint64("fetch_id", fetchID),
		)
		return
	}

	// Enrich message and update UI asynchronously.
	t.currentMessage.ArtistInfo = bio
	updatedMessage := t.currentMessage
	t.mu.Unlock()

	// Use the caller-derived context (already cancelled on shutdown)
	// rather than a detached one, and track via WaitGroup like the
	// parent fetch so Stop() waits for this too.
	if err := t.messenger.UpdatePlaying(parentCtx, &updatedMessage); err != nil {
		t.reportError(parentCtx, "failed to update playing message with async artist bio", err)
	}
}

// fetchArtistInfo tries each getter in order and caches the result.
// Successful lookups are cached for _artistCacheTTL; failures (no
// getter returned a usable bio) are cached only for
// _artistCacheFailureTTL so transient errors don't suppress retries
// for a full day. found=false with a nil ArtistInfoer means "no bio
// available"; the caller decides how to represent that (dummy bio).
func (t *Teletrack) fetchArtistInfo(ctx context.Context, artist string) (ArtistInfoer, bool) {
	if _, recentlyFailed := t.cachedFailedArtistInfoer.Get(artist); recentlyFailed {
		t.logger.Debug("skipping artist lookup: recent failure cached", slog.String("artist", artist))
		return nil, false
	}

	for i, getter := range t.artistGetters {
		artistInfo, err := getter.GetArtistInfo(ctx, artist, []string{"en", "ru"})
		if err != nil {
			t.reportError(
				ctx,
				"artist getter failed",
				err,
				slog.Int("getter_index", i),
				slog.String("artist", artist),
			)
			continue
		}

		if artistInfo.Bio() == "" || artistInfo.BioService() == "" || artistInfo.Link() == "" {
			t.logger.Debug("artist info incomplete, trying next getter",
				slog.Int("getter_index", i),
				slog.String("artist", artist),
			)
			continue
		}

		t.cachedArtistInfoer.Add(artist, artistInfo)
		return artistInfo, true
	}

	t.logger.Info("no artist bio found, using fallback dummy bio", slog.String("artist", artist))
	t.cachedFailedArtistInfoer.Add(artist, struct{}{})
	return nil, false
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
	return shared.TrackProgressSupported(
		track.ProgressMs(),
		track.DurationMs(),
	)
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
