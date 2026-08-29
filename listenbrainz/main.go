// written by: Claude, Gemini
package listenbrainz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/oklookat/teletrack/core"

	"golang.org/x/time/rate"
)

var (
	_lbAPIURL, _ = url.Parse("https://api.listenbrainz.org/1/")
	_mbAPIURL, _ = url.Parse("https://musicbrainz.org/ws/2/")
	_wdAPIURL, _ = url.Parse("https://www.wikidata.org/w/api.php")

	// LB is polled frequently — fail fast, let the next poll cycle retry.
	lbRetryPolicy = retryPolicy{maxRetries: 3, baseDelay: 300 * time.Millisecond, maxDelay: 3 * time.Second}
	// MB is called once per track/artist change and is mid-migration —
	// worth waiting out a longer flaky window.
	mbRetryPolicy = retryPolicy{maxRetries: 7, baseDelay: 500 * time.Millisecond, maxDelay: 15 * time.Second}

	// ErrRepeatedServerError is returned when a 5xx / retry exhaustion error occurs repeatedly consecutively.
	ErrRepeatedServerError = errors.New("repeated server error suppressed")
)

type Config struct {
	// ListenBrainz username to read listens / now-playing for.
	Username string `json:"username"`

	// ListenBrainz user token (Authorization: Token <token>).
	// Optional for reading listens / playing-now (gives relaxed rate limits).
	// Required for submit and other write endpoints.
	Token string `json:"token"`
}

func (c Config) Validate() bool {
	return c.Username != "" && c.Token != ""
}

type retryPolicy struct {
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
}

// Client talks to ListenBrainz for listens/now-playing and to
// MusicBrainz (+ Wikidata/Wikipedia) for artist metadata.
type Client struct {
	HTTP *http.Client

	// MusicBrainz asks for ~1 req/sec.
	mbLimiter *rate.Limiter
	// ListenBrainz / Wikimedia are more lenient, but stay polite.
	lbLimiter *rate.Limiter

	config    *Config
	userAgent string

	cachedInfo *expirable.LRU[string, *ArtistInfo]

	// Error tracking state for suppress consecutive repeated errors
	lastErrMu  sync.Mutex
	lastWas5xx bool
}

// NewClient creates a new ListenBrainz/MusicBrainz client.
func NewClient(cfg *Config, version string) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("nil config")
	}
	if cfg.Username == "" {
		return nil, errors.New("username is required")
	}

	ua := fmt.Sprintf("teletrack/%s (oklocate@gmail.com; https://github.com/oklookat/teletrack)", version)

	return &Client{
		HTTP:       &http.Client{Timeout: 10 * time.Second},
		mbLimiter:  rate.NewLimiter(rate.Every(time.Second), 1),
		lbLimiter:  rate.NewLimiter(rate.Every(time.Second/3), 1),
		config:     cfg,
		userAgent:  ua,
		cachedInfo: expirable.NewLRU[string, *ArtistInfo](50, nil, 10*time.Minute),
	}, nil
}

// handleServerError registers an error and returns ErrRepeatedServerError if it was a consecutive failure.
func (c *Client) handleServerError(err error) error {
	c.lastErrMu.Lock()
	defer c.lastErrMu.Unlock()

	if c.lastWas5xx {
		return ErrRepeatedServerError
	}

	c.lastWas5xx = true
	return err
}

// markSuccess clears the consecutive error state.
func (c *Client) markSuccess() {
	c.lastErrMu.Lock()
	c.lastWas5xx = false
	c.lastErrMu.Unlock()
}

func (c *Client) do(ctx context.Context, limiter *rate.Limiter, req *http.Request, withToken bool) (*http.Response, error) {
	isLB := limiter == c.lbLimiter
	policy := mbRetryPolicy
	if isLB {
		policy = lbRetryPolicy
	}

	var lastErr error
	for attempt := 0; attempt <= policy.maxRetries; attempt++ {
		if err := limiter.Wait(ctx); err != nil {
			return nil, err
		}

		attemptReq := req.Clone(ctx)
		attemptReq.Close = true // force a fresh connection every attempt
		attemptReq.Header.Set("User-Agent", c.userAgent)
		attemptReq.Header.Set("Accept", "application/json")
		if withToken && c.config.Token != "" {
			attemptReq.Header.Set("Authorization", "Token "+c.config.Token)
		}

		resp, err := c.HTTP.Do(attemptReq)
		if err != nil {
			lastErr = err
			if attempt == policy.maxRetries {
				fullErr := fmt.Errorf("after %d attempts: %w", attempt+1, err)
				return nil, c.handleServerError(fullErr)
			}
			if serr := sleepCtx(ctx, backoff(policy, attempt)); serr != nil {
				return nil, serr
			}
			continue
		}

		if isRetryableStatus(resp.StatusCode) {
			delay := retryAfter(resp.Header, policy, attempt)

			// Read up to 200 bytes for diagnostics before closing
			snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
			resp.Body.Close()

			lastErr = fmt.Errorf("status %d from %s: %s", resp.StatusCode, req.URL, string(snippet))
			if attempt == policy.maxRetries {
				givingUpErr := fmt.Errorf("giving up after %d attempts: %w", attempt+1, lastErr)
				return nil, c.handleServerError(givingUpErr)
			}
			if serr := sleepCtx(ctx, delay); serr != nil {
				return nil, serr
			}
			continue
		}

		// Successfully got a non-5xx response
		c.markSuccess()
		return resp, nil
	}

	return nil, c.handleServerError(lastErr)
}

// getJSON issues a GET request against baseURL+path?query and decodes the JSON response body.
func (c *Client) getJSON(ctx context.Context, limiter *rate.Limiter, base *url.URL, path string, query url.Values, out any, withToken bool) error {
	u := *base
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(path, "/")
	u.RawPath = ""
	if query != nil {
		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	resp, err := c.do(ctx, limiter, req, withToken)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("%s: unexpected status %d", u.String(), resp.StatusCode)
		if resp.StatusCode >= 500 {
			return c.handleServerError(err)
		}
		return err
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", u.String(), err)
	}
	return nil
}

// ---------------------------------------------------------------------
// ListenBrainz: now playing / recent listen
// ---------------------------------------------------------------------

type lbListen struct {
	ListenedAt    int64 `json:"listened_at"`
	PlayingNow    bool  `json:"playing_now"`
	TrackMetadata struct {
		ArtistName  string `json:"artist_name"`
		TrackName   string `json:"track_name"`
		ReleaseName string `json:"release_name"`
		MBIDMapping *struct {
			RecordingMBID  string `json:"recording_mbid"`
			ReleaseMBID    string `json:"release_mbid"`
			CAAReleaseMBID string `json:"caa_release_mbid"`
		} `json:"mbid_mapping"`
	} `json:"track_metadata"`
}

type lbListensResponse struct {
	Payload struct {
		Count   int        `json:"count"`
		Listens []lbListen `json:"listens"`
	} `json:"payload"`
}

// GetPlaying returns the user's currently playing track, or the most recent listen if nothing is playing right now.
// If consecutive server errors occur, it returns nil without error.
func (c *Client) GetPlaying(ctx context.Context) (core.TrackInfoer, error) {
	// 1. Prefer the playing-now endpoint.
	var resp lbListensResponse
	path := "user/" + url.PathEscape(c.config.Username) + "/playing-now"
	err := c.getJSON(ctx, c.lbLimiter, _lbAPIURL, path, nil, &resp, true)
	if err != nil {
		if errors.Is(err, ErrRepeatedServerError) {
			return nil, nil // Return empty track on repeated error suppression
		}
		return nil, fmt.Errorf("playing-now: %w", err)
	}
	if len(resp.Payload.Listens) > 0 {
		return c.trackInfoFromListen(resp.Payload.Listens[0]), nil
	}

	// 2. Nothing is playing — fall back to the latest listen.
	q := url.Values{"count": {"1"}}
	path = "user/" + url.PathEscape(c.config.Username) + "/listens"
	err = c.getJSON(ctx, c.lbLimiter, _lbAPIURL, path, q, &resp, true)
	if err != nil {
		if errors.Is(err, ErrRepeatedServerError) {
			return nil, nil // Return empty track on repeated error suppression
		}
		return nil, fmt.Errorf("listens: %w", err)
	}
	if len(resp.Payload.Listens) == 0 {
		return nil, nil
	}
	return c.trackInfoFromListen(resp.Payload.Listens[0]), nil
}

func (c *Client) trackInfoFromListen(l lbListen) *TrackInfo {
	var trackLink, cover string

	if m := l.TrackMetadata.MBIDMapping; m != nil {
		if m.RecordingMBID != "" {
			trackLink = "https://musicbrainz.org/recording/" + m.RecordingMBID
		}
		releaseMBID := m.CAAReleaseMBID
		if releaseMBID == "" {
			releaseMBID = m.ReleaseMBID
		}
		if releaseMBID != "" {
			cover = "https://coverartarchive.org/release/" + releaseMBID + "/front-500"
		}
	}

	t := l.ListenedAt
	when := time.Now()
	if !l.PlayingNow && t > 0 {
		when = time.Unix(t, 0)
	}

	return &TrackInfo{
		playing:   l.PlayingNow,
		artist:    l.TrackMetadata.ArtistName,
		track:     l.TrackMetadata.TrackName,
		trackLink: trackLink,
		coverURL:  cover,
		time:      &when,
	}
}

// ---------------------------------------------------------------------
// MusicBrainz + Wikidata/Wikipedia: artist info
// ---------------------------------------------------------------------

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
func (c *Client) GetArtistInfo(ctx context.Context, artist string, langs []string) (core.ArtistInfoer, error) {
	if info, ok := c.cachedInfo.Get(artist); ok {
		return info, nil
	}

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
	c.cachedInfo.Add(artist, result)

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

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func isRetryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func backoff(p retryPolicy, attempt int) time.Duration {
	d := min(p.baseDelay*time.Duration(1<<attempt), p.maxDelay)
	half := int64(d / 2)
	jitter := time.Duration(rand.Int63n(half + 1))
	return d/2 + jitter
}

func retryAfter(h http.Header, p retryPolicy, attempt int) time.Duration {
	v := h.Get("Retry-After")
	if v == "" {
		return backoff(p, attempt)
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return backoff(p, attempt)
}
