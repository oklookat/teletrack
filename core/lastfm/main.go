package lastfm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/oklookat/teletrack/core/lastfm/lastfmclean"
	"golang.org/x/time/rate"
)

var _apiURL, _ = url.Parse("https://ws.audioscrobbler.com/2.0/")

type Config struct {
	APIKey   string `json:"apiKey"`
	Username string `json:"username"`
}

type ArtistBio struct {
	Name string
	Bio  string
	Link string
}

// Client is a Last.fm API client.
type Client struct {
	HTTP    *http.Client
	limiter *rate.Limiter

	config *Config

	cachedBio *expirable.LRU[string, *ArtistBio]
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

		config:    cfg,
		cachedBio: expirable.NewLRU[string, *ArtistBio](50, nil, 10*time.Minute),
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
func (c *Client) GetArtistBio(ctx context.Context, artist string, langs []string) (*ArtistBio, error) {
	if bio, ok := c.cachedBio.Get(artist); ok {
		return bio, nil
	}

	var gotInfo *ArtistInfo

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

	// bio = shared.TgText(bio)

	result := &ArtistBio{
		Name: gotInfo.Artist.Name,
		Bio:  lastfmclean.NewCleaner().Clean(gotInfo.Artist.Bio.Summary),
		Link: gotInfo.Artist.URL,
	}
	c.cachedBio.Add(artist, result)

	return result, nil
}

// FOR GETTING CURRENT TRACK WITHOUT SPOTIFY
// UserGetRecentTracks fetches recent tracks for a user from Last.fm.
//
// limit (Optional) : The number of results to fetch per page. Defaults to 50. Maximum is 200.
//
// user (Required) : The last.fm username to fetch the recent tracks of.
//
// page (Optional) : The page number to fetch. Defaults to first page.
//
// from (Optional) : Beginning timestamp of a range - only display scrobbles after this time, in UNIX timestamp format (integer number of seconds since 00:00:00, January 1st 1970 UTC). This must be in the UTC time zone.
//
// extended (0|1) (Optional) : Includes extended data in each artist, and whether or not the user has loved each track
//
// to (Optional) : End timestamp of a range - only display scrobbles before this time, in UNIX timestamp format (integer number of seconds since 00:00:00, January 1st 1970 UTC). This must be in the UTC time zone.
// func (c *Client) UserGetRecentTracks(limit *int, page *int, from *time.Time, extended *bool, to *time.Time) (*UserGetRecentTracksResponse, error) {
// 	const method = "user.getRecentTracks"

// 	apiURL := *_apiURL
// 	query := apiURL.Query()
// 	query.Set("method", method)
// 	query.Set("user", c.config.Username)
// 	query.Set("api_key", c.config.APIKey)

// 	if limit != nil {
// 		query.Set("limit", strconv.Itoa(*limit))
// 	}
// 	if page != nil {
// 		query.Set("page", strconv.Itoa(*page))
// 	}
// 	if from != nil {
// 		query.Set("from", fmt.Sprintf("%d", from.UTC().Unix()))
// 	}
// 	if extended != nil {
// 		query.Set("extended", strconv.Itoa(btoi(*extended)))
// 	}
// 	if to != nil {
// 		query.Set("to", fmt.Sprintf("%d", to.UTC().Unix()))
// 	}

// 	query.Set("format", "json")
// 	apiURL.RawQuery = query.Encode()

// 	resp, err := c.HTTP.Get(apiURL.String())
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		var apiErr ApiError
// 		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
// 			return nil, err
// 		}
// 		return nil, apiErr
// 	}

// 	respDec := &UserGetRecentTracksResponse{}
// 	if err := json.NewDecoder(resp.Body).Decode(respDec); err != nil {
// 		return nil, err
// 	}

// 	return respDec, nil
// }

// btoi converts a bool to int (true=1, false=0).
func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
