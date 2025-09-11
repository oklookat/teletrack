package lastfm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// ArtistGetInfo fetches detailed info for an artist from Last.fm.
// lang is an ISO639-2 code (see https://www.loc.gov/standards/iso639-2/php/code_list.php).
func (c *Client) ArtistGetInfo(ctx context.Context, artistName, lang string) (*ArtistInfo, error) {
	const method = "artist.getInfo"

	if artistName == "" {
		return nil, errors.New("artist name is required")
	}
	if c.APIKey == "" {
		return nil, errors.New("API key is required")
	}

	apiURL := *_apiURL
	query := apiURL.Query()

	query.Set("method", method)
	query.Set("api_key", c.APIKey)
	query.Set("artist", artistName)
	if lang != "" {
		query.Set("lang", lang)
	}
	query.Set("format", "json")
	apiURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr ApiError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
			return nil, err
		}
		if apiErr.Code == 6 {
			// Artist not found
			return nil, nil
		}
		return nil, apiErr
	}

	var info ArtistInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &info, nil
}
