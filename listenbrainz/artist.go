// Package listenbrainz implements core.Player and core.ArtistGetter using
// ListenBrainz (now-playing / recent listens) plus MusicBrainz and Wikidata
// for artist biographies.
package listenbrainz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/oklookat/teletrack/core"
)

type mbArtistSearchResponse struct {
	Artists []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Score int    `json:"score"`
	} `json:"artists"`
}

type mbArtistLookupResponse struct {
	ID        string `json:"id"`
	Relations []struct {
		Type string `json:"type"`
		URL  struct {
			Resource string `json:"resource"`
		} `json:"url"`
	} `json:"relations"`
}

type wdSitelinksResponse struct {
	Entities map[string]struct {
		Sitelinks map[string]struct {
			Title string `json:"title"`
		} `json:"sitelinks"`
	} `json:"entities"`
}

type wikiSummaryResponse struct {
	Extract     string `json:"extract"`
	ContentURLs struct {
		Desktop struct {
			Page string `json:"page"`
		} `json:"desktop"`
	} `json:"content_urls"`
}

// GetArtistInfo resolves an artist name to a MusicBrainz entity and tries to find a biography summary via Wikidata.
// If consecutive 5xx or timeout errors occur, it returns an empty biography structure without error.
func (c *Client) GetArtistInfo(ctx context.Context, artist string, langs []string) (core.ArtistInfo, error) {
	mbid, mbErr := c.mbSearchArtist(ctx, artist)
	if mbErr != nil {
		if errors.Is(mbErr, ErrRepeatedServerError) {
			return newArtistInfo("", ""), nil // Return empty bio on repeated error suppression
		}
		return nil, fmt.Errorf("mbSearchArtist: %w", mbErr)
	}
	if mbid == "" {
		return newArtistInfo("", ""), nil
	}

	wikipediaURL, wikidataQID, err := c.mbGetArtistRelations(ctx, mbid)
	if err != nil {
		if errors.Is(err, ErrRepeatedServerError) {
			return newArtistInfo("", ""), nil
		}
		return nil, fmt.Errorf("mbGetArtistRelations: %w", err)
	}

	var extract, pageURL string

	if wikidataQID != "" {
		sitelinks, err := c.wikidataSitelinks(ctx, wikidataQID)
		if err != nil && errors.Is(err, ErrRepeatedServerError) {
			return newArtistInfo("", ""), nil
		}
		if err == nil {
			for _, lang := range langs {
				title, ok := sitelinks[lang+"wiki"]
				if !ok || title == "" {
					continue
				}
				ex, pu, err := c.wikipediaSummary(ctx, lang, title)
				if err != nil {
					continue
				}
				if ex != "" {
					extract, pageURL = ex, pu
					break
				}
			}
		}
	}

	if extract == "" && wikipediaURL != "" {
		if lang, title, ok := parseWikipediaURL(wikipediaURL); ok {
			if ex, pu, err := c.wikipediaSummary(ctx, lang, title); err == nil && ex != "" {
				extract, pageURL = ex, pu
			}
		}
	}

	link := pageURL
	if link == "" {
		link = "https://musicbrainz.org/artist/" + mbid
	}

	result := newArtistInfo(link, strings.TrimSpace(extract))

	return result, nil
}

func (c *Client) mbSearchArtist(ctx context.Context, name string) (string, error) {
	q := url.Values{
		"query": {fmt.Sprintf(`artist:%q`, name)},
		"fmt":   {"json"},
		"limit": {"1"},
	}
	var resp mbArtistSearchResponse
	if err := c.getJSON(ctx, c.mbLimiter, _mbAPIURL, "artist/", q, &resp, false); err != nil {
		return "", err
	}
	if len(resp.Artists) == 0 {
		return "", nil
	}
	return resp.Artists[0].ID, nil
}

func (c *Client) mbGetArtistRelations(ctx context.Context, mbid string) (wikipediaURL, wikidataQID string, err error) {
	q := url.Values{"inc": {"url-rels"}, "fmt": {"json"}}
	var resp mbArtistLookupResponse
	if err := c.getJSON(ctx, c.mbLimiter, _mbAPIURL, "artist/"+mbid, q, &resp, false); err != nil {
		return "", "", err
	}
	for _, rel := range resp.Relations {
		switch rel.Type {
		case "wikidata":
			if idx := strings.LastIndex(rel.URL.Resource, "/"); idx != -1 {
				wikidataQID = rel.URL.Resource[idx+1:]
			}
		case "wikipedia":
			wikipediaURL = rel.URL.Resource
		}
	}
	return wikipediaURL, wikidataQID, nil
}

func (c *Client) wikidataSitelinks(ctx context.Context, qid string) (map[string]string, error) {
	q := url.Values{
		"action": {"wbgetentities"},
		"ids":    {qid},
		"props":  {"sitelinks"},
		"format": {"json"},
	}
	var resp wdSitelinksResponse
	if err := c.getJSON(ctx, c.lbLimiter, _wdAPIURL, "", q, &resp, false); err != nil {
		return nil, err
	}
	entity, ok := resp.Entities[qid]
	if !ok {
		return nil, nil
	}
	out := make(map[string]string, len(entity.Sitelinks))
	for site, sl := range entity.Sitelinks {
		out[site] = sl.Title
	}
	return out, nil
}

func (c *Client) wikipediaSummary(ctx context.Context, lang, title string) (extract, pageURL string, err error) {
	slug := url.PathEscape(strings.ReplaceAll(title, " ", "_"))
	rawURL := fmt.Sprintf("https://%s.wikipedia.org/api/rest_v1/page/summary/%s", lang, slug)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", err
	}

	resp, err := c.do(ctx, c.lbLimiter, req, false)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("%s: unexpected status %d", rawURL, resp.StatusCode)
	}

	var out wikiSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("decode %s: %w", rawURL, err)
	}
	return out.Extract, out.ContentURLs.Desktop.Page, nil
}

func parseWikipediaURL(raw string) (lang, title string, ok bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", false
	}
	host := strings.TrimSuffix(u.Host, ".wikipedia.org")
	if host == u.Host || host == "" {
		return "", "", false
	}
	const prefix = "/wiki/"
	if !strings.HasPrefix(u.Path, prefix) {
		return "", "", false
	}
	title, err = url.PathUnescape(strings.TrimPrefix(u.Path, prefix))
	if err != nil || title == "" {
		return "", "", false
	}
	return host, title, true
}

