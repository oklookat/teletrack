// Package lastfm implements core.Player and core.ArtistGetter using the
// Last.fm API (recent tracks and artist.getInfo).
package lastfm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/oklookat/teletrack/core"
	"github.com/oklookat/teletrack/lastfm/lastfmclean"
	"golang.org/x/time/rate"
)

const (
	_artistBioService = "Last.fm"
	_trackLinkService = "Last.fm"
)

var (
	_apiURL, _ = url.Parse("https://ws.audioscrobbler.com/2.0/")
)

type (
	RecentTrack struct {
		Artist     string
		Track      string
		PlayingNow bool
		// Can be empty.
		Cover string
	}
)

// Client is a Last.fm API client.
type Client struct {
	HTTP    *http.Client
	limiter *rate.Limiter

	config *Config
}

// NewClient creates a new Last.fm API client.
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("nil config")
	}
	if cfg.Username == "" {
		return nil, errors.New("user is required")
	}
	if cfg.APIKey == "" {
		return nil, errors.New("API key is required")
	}
	return &Client{
		HTTP:    &http.Client{Timeout: 10 * time.Second},
		limiter: rate.NewLimiter(rate.Every(time.Second), 1),
		config:  cfg,
	}, nil
}

// Service implements core.ArtistGetter.
func (*Client) Service() string {
	return _artistBioService
}

func (c *Client) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if c == nil {
		return nil, errors.New("client is nil")
	}
	if req == nil {
		return nil, errors.New("request is nil")
	}
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	req = req.WithContext(ctx)

	if c.HTTP == nil {
		return nil, errors.New("http client is nil")
	}

	return c.HTTP.Do(req)
}

// GetArtistInfo retrieves bio information using fallback languages.
func (c *Client) GetArtistInfo(ctx context.Context, artist string, langs []string) (core.ArtistInfo, error) {
	if c == nil {
		return nil, errors.New("client is nil")
	}

	var gotInfo *ArtistFull

	for _, lang := range langs {
		info, err := c.artistGetInfo(ctx, artist, lang)
		if err != nil {
			return nil, fmt.Errorf("artistGetInfo: %w", err)
		}
		if info != nil {
			gotInfo = info
			break
		}
	}

	if gotInfo == nil {
		return nil, errors.New("artist info not found")
	}

	summary := ""
	cleaner := lastfmclean.NewCleaner()
	if cleaner != nil {
		summary = cleaner.Clean(gotInfo.Artist.Bio.Summary)
	} else {
		summary = gotInfo.Artist.Bio.Summary
	}

	return newArtistInfo(gotInfo.Artist.URL, summary), nil
}

func (c *Client) GetPlaying(ctx context.Context) (core.Track, error) {
	if c == nil {
		return nil, errors.New("client is nil")
	}

	limit := 1
	extended := true
	resp, err := c.userGetRecentTracks(ctx, &limit, nil, nil, &extended, nil)
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.Recenttracks.Track) == 0 {
		return nil, nil
	}

	track := resp.Recenttracks.Track[0]
	if track == nil {
		return nil, errors.New("first track is nil")
	}

	if err := track.Validate(); err != nil {
		return nil, err
	}

	playingNow := track.IsNowPlaying()

	var cover string
	if len(track.Image) > 0 {
		biggestImage := track.Image[len(track.Image)-1]
		urld, err := url.Parse(biggestImage.Text)
		if err == nil && urld != nil && strings.EqualFold(urld.Scheme, "https") {
			cover = urld.String()
		}
	}

	var trackTime *time.Time
	if track.Date != nil {
		trackTime = track.Date.ToTime()
	}
	if trackTime == nil {
		now := time.Now()
		trackTime = &now
	}

	return &TrackInfo{
		playing:   playingNow,
		artist:    track.ArtistName(),
		track:     track.Name,
		trackLink: track.URL,
		coverURL:  cover,
		time:      trackTime,
	}, nil
}

// btoi converts a bool to int (true=1, false=0).
func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
