package lastfm

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
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
func (c *Client) UserGetTopTracks(user string, period *UserGetTopTracksPeriod, limit *int, page *int) (*UserGetTopTracksResponse, error) {
	const method = "user.getTopTracks"

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
	if period != nil {
		query.Set("period", string(*period))
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

	respDec := &UserGetTopTracksResponse{}
	if err := json.NewDecoder(resp.Body).Decode(respDec); err != nil {
		return nil, err
	}

	return respDec, nil
}

// UserGetTopArtists fetches top artists for a user from Last.fm.
func (c *Client) UserGetTopArtists(user string, period *UserGetTopTracksPeriod, limit *int, page *int) (*UserGetTopArtistsResponse, error) {
	const method = "user.getTopArtists"

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
	if period != nil {
		query.Set("period", string(*period))
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

	respDec := &UserGetTopArtistsResponse{}
	if err := json.NewDecoder(resp.Body).Decode(respDec); err != nil {
		return nil, err
	}

	return respDec, nil
}
