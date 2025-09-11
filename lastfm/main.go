package lastfm

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var _apiURL, _ = url.Parse("https://ws.audioscrobbler.com/2.0/")

// Client is a Last.fm API client.
type Client struct {
	APIKey string
	HTTP   *http.Client
}

// NewClient creates a new Last.fm API client.
func NewClient(apiKey string) *Client {
	return &Client{
		APIKey: apiKey,
		HTTP:   &http.Client{Timeout: 10 * time.Second},
	}
}

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
//
// api_key (Required) : A Last.fm API key.
func (c *Client) UserGetRecentTracks(user string, limit *int, page *int, from *time.Time, extended *bool, to *time.Time) (*UserGetRecentTracksResponse, error) {
	const method = "user.getRecentTracks"

	if user == "" {
		return nil, errors.New("user is required")
	}
	if c.APIKey == "" {
		return nil, errors.New("API key is required")
	}

	apiURL := *_apiURL
	query := apiURL.Query()
	query.Set("method", method)
	query.Set("user", user)
	query.Set("api_key", c.APIKey)

	if limit != nil {
		query.Set("limit", strconv.Itoa(*limit))
	}
	if page != nil {
		query.Set("page", strconv.Itoa(*page))
	}
	if from != nil {
		query.Set("from", fmt.Sprintf("%d", from.UTC().Unix()))
	}
	if extended != nil {
		query.Set("extended", strconv.Itoa(btoi(*extended)))
	}
	if to != nil {
		query.Set("to", fmt.Sprintf("%d", to.UTC().Unix()))
	}

	query.Set("format", "json")
	apiURL.RawQuery = query.Encode()

	resp, err := c.HTTP.Get(apiURL.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr ApiError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
			return nil, err
		}
		return nil, apiErr
	}

	respDec := &UserGetRecentTracksResponse{}
	if err := json.NewDecoder(resp.Body).Decode(respDec); err != nil {
		return nil, err
	}

	return respDec, nil
}

// btoi converts a bool to int (true=1, false=0).
func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
