package lastfm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/oklookat/teletrack/core"
	"github.com/oklookat/teletrack/lastfm/lastfmclean"
	"golang.org/x/time/rate"
)

var _apiURL, _ = url.Parse("https://ws.audioscrobbler.com/2.0/")

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

	cachedInfo *expirable.LRU[string, *ArtistInfo]
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

		config:     cfg,
		cachedInfo: expirable.NewLRU[string, *ArtistInfo](50, nil, 10*time.Minute),
	}, nil
}

func (c *Client) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)

	return c.HTTP.Do(req)
}

// First lang is preferred. Other langs for fallback if first lang doesnt have bio.
//
// Langs format:
//
// ISO639-2 code (see https://www.loc.gov/standards/iso639-2/php/code_list.php
func (c *Client) GetArtistInfo(ctx context.Context, artist string, langs []string) (core.ArtistInfoer, error) {
	if bio, ok := c.cachedInfo.Get(artist); ok {
		return bio, nil
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
		return nil, errors.New("gotInfo == nil")
	}

	result := newArtistInfo(gotInfo.Artist.URL, lastfmclean.NewCleaner().Clean(gotInfo.Artist.Bio.Summary))
	c.cachedInfo.Add(artist, result)

	return result, nil
}

func (c *Client) GetPlaying(ctx context.Context) (core.TrackInfoer, error) {
	resp, err := c.userGetRecentTracks(ctx, new(1), nil, nil, new(true), nil)
	if err != nil {
		return nil, err
	}
	if len(resp.Recenttracks.Track) == 0 {
		return nil, nil
	}
	track := resp.Recenttracks.Track[0]
	if err := track.Validate(); err != nil {
		return nil, err
	}

	var playingNow bool
	if track.Attr.Nowplaying != nil {
		playingNow = *track.Attr.Nowplaying
	}

	var cover string
	if len(track.Image) > 0 {
		biggestImage := track.Image[len(track.Image)-1]
		urld, err := url.Parse(biggestImage.Text)
		if err == nil && strings.ToLower(urld.Scheme) == "https" {
			cover = urld.String()
		}
	}

	trackTime := track.Date.ToTime()
	if trackTime == nil {
		trackTime = new(time.Now())
	}

	return &TrackInfo{
		playing:   playingNow,
		artist:    track.Artist.Name,
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
