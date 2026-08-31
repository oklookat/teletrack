package core

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/oklookat/teletrack/shared"
)

// --- fakes ---

type fakeTrack struct {
	id, artist, track string
	playing           bool
	progress, duration *int
	at                *time.Time
}

func (f *fakeTrack) ID() string              { return f.id }
func (f *fakeTrack) Playing() bool           { return f.playing }
func (f *fakeTrack) Artist() string          { return f.artist }
func (f *fakeTrack) Track() string           { return f.track }
func (f *fakeTrack) TrackLink() string       { return "" }
func (f *fakeTrack) TrackLinkService() string { return "" }
func (f *fakeTrack) CoverURL() string        { return "" }
func (f *fakeTrack) ProgressMs() *int        { return f.progress }
func (f *fakeTrack) DurationMs() *int        { return f.duration }
func (f *fakeTrack) Time() *time.Time        { return f.at }

type fakePlayer struct {
	mu    sync.Mutex
	track Track
	err   error
}

func (p *fakePlayer) GetPlaying(context.Context) (Track, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.track, p.err
}

func (p *fakePlayer) set(track Track) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.track = track
}

type fakeRenderer struct {
	mu       sync.Mutex
	playing  int
	idle     int
	lastPlay *PlayingMessage
	lastIdle *PlayingMessage
}

func (m *fakeRenderer) UpdatePlaying(_ context.Context, msg *PlayingMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.playing++
	m.lastPlay = msg
	return nil
}

func (m *fakeRenderer) UpdateIdle(_ context.Context, msg *PlayingMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idle++
	m.lastIdle = msg
	return nil
}

type memCache struct {
	mu   sync.Mutex
	data map[string][]byte
	fail map[string]bool
}

func newMemCache() *memCache {
	return &memCache{data: map[string][]byte{}, fail: map[string]bool{}}
}

func (c *memCache) Get(_ context.Context, key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.data[key]
	return v, ok
}

func (c *memCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
	delete(c.fail, key)
	return nil
}

func (c *memCache) SetFailed(_ context.Context, key string, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fail[key] = true
	delete(c.data, key)
	return nil
}

func (c *memCache) IsFailed(_ context.Context, key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.fail[key]
}

func (c *memCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	delete(c.fail, key)
	return nil
}

func (c *memCache) Close() error { return nil }

// --- tests ---

func TestUpdatePlaybackState_IdleWhenNil(t *testing.T) {
	tt := newTestTeletrack(t, nil, nil)
	idle, wasIdle := tt.updatePlaybackState(nil)
	if !idle {
		t.Fatal("nil track should be idle")
	}
	if !wasIdle {
		t.Fatal("initial state is idle, wasIdle should be true")
	}
}

func TestUpdatePlaybackState_PausedTicks(t *testing.T) {
	tt := newTestTeletrack(t, nil, nil)
	track := &fakeTrack{id: "a", playing: false}

	for i := 0; i < pausedTicksThreshold-1; i++ {
		idle, _ := tt.updatePlaybackState(track)
		if idle {
			t.Fatalf("tick %d should not yet be idle", i)
		}
	}
	idle, _ := tt.updatePlaybackState(track)
	if !idle {
		t.Fatal("should be idle after pausedTicksThreshold")
	}
}

func TestUpdatePlaybackState_PlayingResetsPause(t *testing.T) {
	tt := newTestTeletrack(t, nil, nil)
	paused := &fakeTrack{id: "a", playing: false}
	tt.updatePlaybackState(paused)
	tt.updatePlaybackState(paused)

	playing := &fakeTrack{id: "a", playing: true}
	idle, _ := tt.updatePlaybackState(playing)
	if idle {
		t.Fatal("playing track should not be idle")
	}
	if tt.playback.pausedTicks != 0 {
		t.Fatalf("pausedTicks = %d, want 0", tt.playback.pausedTicks)
	}
}

func TestHandleTick_NothingPlayingCallsIdle(t *testing.T) {
	player := &fakePlayer{}
	msg := &fakeRenderer{}
	tt := newTestTeletrack(t, []Player{player}, msg)

	if err := tt.handleTick(context.Background()); err != nil {
		t.Fatalf("handleTick: %v", err)
	}
	msg.mu.Lock()
	defer msg.mu.Unlock()
	if msg.idle != 1 {
		t.Fatalf("idle updates = %d, want 1", msg.idle)
	}
	if msg.playing != 0 {
		t.Fatalf("playing updates = %d, want 0", msg.playing)
	}
}

func TestHandleTick_NewTrackCallsPlaying(t *testing.T) {
	track := &fakeTrack{
		id:      shared.GenerateTrackID("Artist", "Song"),
		artist:  "Artist",
		track:   "Song",
		playing: true,
	}
	player := &fakePlayer{track: track}
	msg := &fakeRenderer{}
	tt := newTestTeletrack(t, []Player{player}, msg)

	if err := tt.handleTick(context.Background()); err != nil {
		t.Fatalf("handleTick: %v", err)
	}
	// Allow async artist fetch goroutine to finish or be discarded.
	tt.wg.Wait()

	msg.mu.Lock()
	defer msg.mu.Unlock()
	if msg.playing < 1 {
		t.Fatalf("playing updates = %d, want >= 1", msg.playing)
	}
}

func TestGetPlaying_SkipsEmptyID(t *testing.T) {
	bad := &fakeTrack{id: "", artist: "A", track: "T", playing: true}
	good := &fakeTrack{id: "good", artist: "B", track: "U", playing: true}
	p1 := &fakePlayer{track: bad}
	p2 := &fakePlayer{track: good}
	tt := newTestTeletrack(t, []Player{p1, p2}, &fakeRenderer{})

	got, err := tt.getPlaying(context.Background())
	if err != nil {
		t.Fatalf("getPlaying: %v", err)
	}
	if got == nil || got.ID() != "good" {
		t.Fatalf("got %#v, want good", got)
	}
}

func TestSupportsProgress(t *testing.T) {
	p, d := 10, 100
	if !supportsProgress(&fakeTrack{progress: &p, duration: &d}) {
		t.Fatal("expected supported")
	}
	if supportsProgress(&fakeTrack{}) {
		t.Fatal("expected unsupported")
	}
	zero := 0
	if supportsProgress(&fakeTrack{progress: &p, duration: &zero}) {
		t.Fatal("zero duration unsupported")
	}
}

type fakeArtistInfo struct {
	link, bio, service string
}

func (f fakeArtistInfo) Link() string       { return f.link }
func (f fakeArtistInfo) Bio() string        { return f.bio }
func (f fakeArtistInfo) BioService() string { return f.service }

type fakeArtistGetter struct {
	mu      sync.Mutex
	calls   int
	info    ArtistInfo
	err     error
	blockCh chan struct{} // if non-nil, blocks until closed
}

func (g *fakeArtistGetter) Service() string { return "test" }

func (g *fakeArtistGetter) GetArtistInfo(ctx context.Context, artist string, langs []string) (ArtistInfo, error) {
	g.mu.Lock()
	g.calls++
	g.mu.Unlock()
	if g.blockCh != nil {
		select {
		case <-g.blockCh:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if g.err != nil {
		return nil, g.err
	}
	return g.info, nil
}

func (g *fakeArtistGetter) callCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls
}

func TestOnNewTrackPlaying_AsyncBioUpdatesMessage(t *testing.T) {
	track := &fakeTrack{
		id:      shared.GenerateTrackID("BioArtist", "Song"),
		artist:  "BioArtist",
		track:   "Song",
		playing: true,
	}
	player := &fakePlayer{track: track}
	msg := &fakeRenderer{}
	getter := &fakeArtistGetter{
		info: fakeArtistInfo{
			link:    "https://example.com/artist",
			bio:     "A short bio.",
			service: "TestWiki",
		},
	}
	tt := newTestTeletrack(t, []Player{player}, msg, getter)

	if err := tt.handleTick(context.Background()); err != nil {
		t.Fatalf("handleTick: %v", err)
	}
	tt.wg.Wait()

	msg.mu.Lock()
	defer msg.mu.Unlock()
	// At least: initial update (dummy bio) + async bio update.
	if msg.playing < 2 {
		t.Fatalf("playing updates = %d, want >= 2 (initial + bio)", msg.playing)
	}
	if msg.lastPlay == nil || msg.lastPlay.ArtistInfo == nil {
		t.Fatal("expected last playing message with artist info")
	}
	if msg.lastPlay.ArtistInfo.Bio() != "A short bio." {
		t.Fatalf("bio = %q", msg.lastPlay.ArtistInfo.Bio())
	}
	if getter.callCount() != 1 {
		t.Fatalf("getter calls = %d, want 1", getter.callCount())
	}
}

func TestOnNewTrackPlaying_CacheHitSkipsGetter(t *testing.T) {
	track := &fakeTrack{
		id:      shared.GenerateTrackID("Cached", "Song"),
		artist:  "Cached",
		track:   "Song",
		playing: true,
	}
	player := &fakePlayer{track: track}
	msg := &fakeRenderer{}
	getter := &fakeArtistGetter{
		info: fakeArtistInfo{link: "https://x", bio: "from getter", service: "G"},
	}
	cache := newMemCache()
	bio, err := artistInfoToBytes(fakeArtistInfo{link: "https://c", bio: "from cache", service: "C"})
	if err != nil {
		t.Fatal(err)
	}
	_ = cache.Set(context.Background(), artistCachePrefix+"Cached", bio, 0)

	tt, err := New([]Player{player}, []ArtistGetter{getter}, cache, []Renderer{msg}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if err := tt.handleTick(context.Background()); err != nil {
		t.Fatal(err)
	}
	tt.wg.Wait()

	if getter.callCount() != 0 {
		t.Fatalf("getter should not be called on cache hit, got %d", getter.callCount())
	}
	msg.mu.Lock()
	defer msg.mu.Unlock()
	if msg.lastPlay == nil || msg.lastPlay.ArtistInfo.Bio() != "from cache" {
		t.Fatalf("expected cached bio, got %#v", msg.lastPlay)
	}
}

func TestFetchArtistInfoAsync_DiscardedWhenTrackChanges(t *testing.T) {
	block := make(chan struct{})
	slow := &fakeArtistGetter{
		info:    fakeArtistInfo{link: "https://slow", bio: "slow bio", service: "S"},
		blockCh: block,
	}
	msg := &fakeRenderer{}
	tt := newTestTeletrack(t, nil, msg, slow)

	t1 := &fakeTrack{id: "t1", artist: "A1", track: "S1", playing: true}
	if err := tt.onNewTrackPlaying(context.Background(), t1); err != nil {
		t.Fatal(err)
	}

	// Switch track before slow getter returns.
	t2 := &fakeTrack{id: "t2", artist: "A2", track: "S2", playing: true}
	if err := tt.onNewTrackPlaying(context.Background(), t2); err != nil {
		t.Fatal(err)
	}

	close(block) // unblock both fetches
	tt.wg.Wait()

	msg.mu.Lock()
	defer msg.mu.Unlock()
	// Final message should still be for t2, not overwritten by stale t1 bio.
	if msg.lastPlay == nil || msg.lastPlay.TrackInfo == nil {
		t.Fatal("missing last play message")
	}
	if msg.lastPlay.TrackInfo.ID() != "t2" {
		t.Fatalf("track id = %q, want t2", msg.lastPlay.TrackInfo.ID())
	}
}

func newTestTeletrack(t *testing.T, players []Player, renderer Renderer, getters ...ArtistGetter) *Teletrack {
	t.Helper()
	var renderers []Renderer
	if renderer != nil {
		renderers = []Renderer{renderer}
	}
	tt, err := New(players, getters, newMemCache(), renderers, slog.Default())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return tt
}

func TestBroadcast_ParallelRenderers(t *testing.T) {
	var started sync.WaitGroup
	var release sync.WaitGroup
	started.Add(2)
	release.Add(1)

	block := func() {
		started.Done()
		release.Wait()
	}

	r1 := &blockingRenderer{onPlay: block}
	r2 := &blockingRenderer{onPlay: block}
	tt, err := New(nil, nil, newMemCache(), []Renderer{r1, r2}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- tt.broadcastPlaying(context.Background(), &PlayingMessage{})
	}()

	// Both renderers must have entered UpdatePlaying before either finishes.
	waitCh := make(chan struct{})
	go func() {
		started.Wait()
		close(waitCh)
	}()
	select {
	case <-waitCh:
	case <-time.After(2 * time.Second):
		t.Fatal("renderers did not start in parallel")
	}

	release.Done()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast did not finish")
	}
}

type blockingRenderer struct {
	onPlay func()
}

func (b *blockingRenderer) UpdatePlaying(context.Context, *PlayingMessage) error {
	if b.onPlay != nil {
		b.onPlay()
	}
	return nil
}
func (b *blockingRenderer) UpdateIdle(context.Context, *PlayingMessage) error { return nil }
