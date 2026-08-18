package lastfm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// UserGetTopTracksPeriod defines the period for top tracks/artists queries.
type UserGetTopTracksPeriod string

const (
	UserGetTopTracksPeriodOverall UserGetTopTracksPeriod = "overall"
	UserGetTopTracksPeriod7Day    UserGetTopTracksPeriod = "7day"
	UserGetTopTracksPeriod1Month  UserGetTopTracksPeriod = "1month"
	UserGetTopTracksPeriod3Month  UserGetTopTracksPeriod = "3month"
	UserGetTopTracksPeriod6Month  UserGetTopTracksPeriod = "6month"
	UserGetTopTracksPeriod12Month UserGetTopTracksPeriod = "12month"
)

// UserGetTopTracks fetches top tracks for a user from Last.fm.
func (c *Client) UserGetTopTracks(ctx context.Context, user string, period *UserGetTopTracksPeriod, limit *int, page *int) (*UserGetTopTracksResponse, error) {
	const method = "user.getTopTracks"

	apiURL := *_apiURL
	query := apiURL.Query()
	query.Set("method", method)
	query.Set("user", user)
	query.Set("api_key", c.config.APIKey)

	if limit != nil {
		query.Set("limit", strconv.Itoa(*limit))
	}
	if page != nil {
		query.Set("page", strconv.Itoa(*page))
	}
	if period != nil {
		query.Set("period", string(*period))
	}

	query.Set("format", "json")
	apiURL.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, apiURL.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, req)
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

	respDec := &UserGetTopTracksResponse{}
	if err := json.NewDecoder(resp.Body).Decode(respDec); err != nil {
		return nil, err
	}

	return respDec, nil
}

// UserGetTopArtists fetches top artists for a user from Last.fm.
func (c *Client) UserGetTopArtists(ctx context.Context, user string, period *UserGetTopTracksPeriod, limit *int, page *int) (*UserGetTopArtistsResponse, error) {
	const method = "user.getTopArtists"

	apiURL := *_apiURL
	query := apiURL.Query()
	query.Set("method", method)
	query.Set("user", user)
	query.Set("api_key", c.config.APIKey)

	if limit != nil {
		query.Set("limit", strconv.Itoa(*limit))
	}
	if page != nil {
		query.Set("page", strconv.Itoa(*page))
	}
	if period != nil {
		query.Set("period", string(*period))
	}

	query.Set("format", "json")
	apiURL.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, apiURL.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, req)
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

	respDec := &UserGetTopArtistsResponse{}
	if err := json.NewDecoder(resp.Body).Decode(respDec); err != nil {
		return nil, err
	}

	return respDec, nil
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
func (c *Client) userGetRecentTracks(ctx context.Context, limit *int, page *int, from *time.Time, extended *bool, to *time.Time) (*UserGetRecentTracksResponse, error) {
	const method = "user.getRecentTracks"

	apiURL := *_apiURL
	query := apiURL.Query()
	query.Set("method", method)
	query.Set("user", c.config.Username)
	query.Set("api_key", c.config.APIKey)

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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(ctx, req)
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
