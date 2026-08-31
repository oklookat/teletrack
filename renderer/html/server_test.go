package html

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/oklookat/teletrack/core"
	"github.com/oklookat/teletrack/renderer/api"
	"github.com/oklookat/teletrack/shared"
)

type stubTrack struct {
	id, artist, track, cover, link string
	playing                        bool
	progress, duration             *int
}

func (t *stubTrack) ID() string               { return t.id }
func (t *stubTrack) Playing() bool            { return t.playing }
func (t *stubTrack) Artist() string           { return t.artist }
func (t *stubTrack) Track() string            { return t.track }
func (t *stubTrack) TrackLink() string        { return t.link }
func (t *stubTrack) TrackLinkService() string { return "Test" }
func (t *stubTrack) CoverURL() string         { return t.cover }
func (t *stubTrack) ProgressMs() *int         { return t.progress }
func (t *stubTrack) DurationMs() *int         { return t.duration }
func (t *stubTrack) Time() *time.Time         { return nil }

func TestServer_StateAndUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sharedAPI := api.New()
	s, err := Start(ctx, Config{Addr: "127.0.0.1:0"}, sharedAPI)
	if err != nil {
		t.Fatal(err)
	}
	if s.API() != sharedAPI {
		t.Fatal("html should use the shared api.Renderer")
	}

	base := "http://" + s.Addr()
	playingURL := base + api.DefaultPathPrefix + "/playing"

	resp, err := http.Get(playingURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var state api.State
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if !state.Idle || state.Playing {
		t.Fatalf("expected idle initially, got %+v", state)
	}

	prog, dur := 30_000, 180_000
	msg := &core.PlayingMessage{
		TrackInfo: &stubTrack{
			id:       shared.GenerateTrackID("A", "T"),
			artist:   "Artist",
			track:    "Title",
			playing:  true,
			cover:    "https://example.com/c.jpg",
			link:     "https://example.com/t",
			progress: &prog,
			duration: &dur,
		},
		Emoji:         ":)",
		Watermark:     "teletrack",
		WatermarkLink: "https://github.com/oklookat/teletrack",
	}
	if err := sharedAPI.UpdatePlaying(ctx, msg); err != nil {
		t.Fatal(err)
	}

	resp2, err := http.Get(playingURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if err := json.NewDecoder(resp2.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if state.Idle || !state.Playing {
		t.Fatalf("expected playing, got %+v", state)
	}
	if state.Track == nil || state.Track.Artist != "Artist" || state.Track.Title != "Title" {
		t.Fatalf("unexpected track: %+v", state.Track)
	}

	htmlResp, err := http.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer htmlResp.Body.Close()
	body, _ := io.ReadAll(htmlResp.Body)
	if len(body) < 100 || htmlResp.Header.Get("Content-Type") == "" {
		t.Fatal("expected HTML body")
	}
}

func TestServer_HTMLOnlyCreatesPrivateAPI(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := Start(ctx, Config{Addr: "127.0.0.1:0"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.API() == nil {
		t.Fatal("expected private api.Renderer when none was shared")
	}

	resp, err := http.Get("http://" + s.Addr() + api.DefaultPathPrefix + "/playing")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}
