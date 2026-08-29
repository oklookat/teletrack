package lastfm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// ArtistGetInfo fetches detailed info for an artist from Last.fm.
// lang is an ISO639-2 code (see https://www.loc.gov/standards/iso639-2/php/code_list.php).
func (c *Client) artistGetInfo(ctx context.Context, artistName, lang string) (*ArtistFull, error) {
	const method = "artist.getInfo"

	if artistName == "" {
		return nil, errors.New("artist name is required")
	}
	if c.config.APIKey == "" {
		return nil, errors.New("API key is required")
	}

	apiURL := *_apiURL
	query := apiURL.Query()

	query.Set("method", method)
	query.Set("api_key", c.config.APIKey)
	query.Set("artist", artistName)
	if lang != "" {
		query.Set("lang", lang)
	}
	query.Set("format", "json")
	apiURL.RawQuery = query.Encode()

	apiUrl := apiURL.String()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiUrl, nil)
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
		if apiErr.Code == 6 {
			// Artist not found
			return nil, nil
		}
		return nil, apiErr
	}

	var full ArtistFull
	if err := json.NewDecoder(resp.Body).Decode(&full); err != nil {
		return nil, err
	}

	return &full, nil
}
