package lastfm

import (
	"encoding/json"
	"net/http"
	"time"
)

// lang (iso639-2) - https://www.loc.gov/standards/iso639-2/php/code_list.php
func (c Client) ArtistGetInfo(artistName, lang string) (*ArtistInfo, error) {
	const method = "artist.getInfo"

	apiURL := *_apiURL
	query := apiURL.Query()

	query.Set("method", method)
	query.Set("api_key", c.apiKey)
	query.Set("artist", artistName)
	query.Set("lang", lang)

	query.Set("format", "json")
	apiURL.RawQuery = query.Encode()

	hClient := http.DefaultClient
	hClient.Timeout = 5 * time.Second

	resp, err := hClient.Get(apiURL.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var apiErr ApiError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
			return nil, err
		}
		if apiErr.Code == 6 {
			// Artist not found.
			return nil, nil
		}
		return nil, apiErr
	}

	respDec := &ArtistInfo{}
	if err := json.NewDecoder(resp.Body).Decode(respDec); err != nil {
		return nil, err
	}

	return respDec, err
}
