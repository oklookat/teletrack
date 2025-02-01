package lastfm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var _apiURL, _ = url.Parse("https://ws.audioscrobbler.com/2.0/")

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
	}
}

type Client struct {
	apiKey string
}

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
func (c Client) UserGetRecentTracks(user string, limit *int, page *int, from *time.Time, extended *bool, to *time.Time) (*UserGetRecentTracksResponse, error) {
	const method = "user.getRecentTracks"

	apiURL := *_apiURL
	query := apiURL.Query()

	query.Set("method", method)
	query.Set("user", user)
	query.Set("api_key", c.apiKey)

	apiURL.RawQuery = query.Encode()

	if limit != nil {
		query.Set("limit", strconv.Itoa(*limit))
	}
	if page != nil {
		query.Set("page", strconv.Itoa(*limit))
	}
	if from != nil {
		query.Set("from", fmt.Sprintf("%d", from.UTC().UnixMilli()))
	}
	if extended != nil {
		query.Set("extended", strconv.Itoa(btoi(*extended)))
	}
	if to != nil {
		query.Set("to", fmt.Sprintf("%d", to.UTC().UnixMilli()))
	}

	query.Set("format", "json")
	apiURL.RawQuery = query.Encode()

	resp, err := http.Get(apiURL.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
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

	return respDec, err
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
