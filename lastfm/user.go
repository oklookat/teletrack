package lastfm

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type UserGetTopTracksPeriod string

const (
	UserGetTopTracksPeriodOverall UserGetTopTracksPeriod = "overall"
	UserGetTopTracksPeriod7Day    UserGetTopTracksPeriod = "7day"
	UserGetTopTracksPeriod1Month  UserGetTopTracksPeriod = "1month"
	UserGetTopTracksPeriod3Month  UserGetTopTracksPeriod = "3month"
	UserGetTopTracksPeriod6Month  UserGetTopTracksPeriod = "6month"
	UserGetTopTracksPeriod12Month UserGetTopTracksPeriod = "12month"
)

func (c Client) UserGetTopTracks(user string, period *UserGetTopTracksPeriod, limit *int, page *int) (*UserGetTopTracksResponse, error) {
	const method = "user.getTopTracks"

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
	if period != nil {
		uPtr := *period
		query.Set("period", string(uPtr))
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

	respDec := &UserGetTopTracksResponse{}
	if err := json.NewDecoder(resp.Body).Decode(respDec); err != nil {
		return nil, err
	}

	return respDec, err
}

func (c Client) UserGetTopArtists(user string, period *UserGetTopTracksPeriod, limit *int, page *int) (*UserGetTopArtistsResponse, error) {
	const method = "user.getTopArtists"

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
	if period != nil {
		uPtr := *period
		query.Set("period", string(uPtr))
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

	respDec := &UserGetTopArtistsResponse{}
	if err := json.NewDecoder(resp.Body).Decode(respDec); err != nil {
		return nil, err
	}

	return respDec, err
}
