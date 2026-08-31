// Package listenbrainz implements core.Player and core.ArtistGetter using
// ListenBrainz (now-playing / recent listens) plus MusicBrainz and Wikidata
// for artist biographies.
package listenbrainz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/oklookat/teletrack/shared"
	"golang.org/x/time/rate"
)

const (
	_artistBioService string = "Wikipedia"
	_trackLinkService string = "ListenBrainz"
)

var (
	_ua          = fmt.Sprintf("teletrack/%s (oklocate@gmail.com; https://github.com/oklookat/teletrack)", shared.Version)
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

	// Error tracking state for suppress consecutive repeated errors
	lastErrMu  sync.Mutex
	lastWas5xx bool
}

// NewClient creates a new ListenBrainz/MusicBrainz client.
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("nil config")
	}
	if cfg.Username == "" {
		return nil, errors.New("username is required")
	}
	return &Client{
		HTTP:      &http.Client{Timeout: 10 * time.Second},
		mbLimiter: rate.NewLimiter(rate.Every(time.Second), 1),
		lbLimiter: rate.NewLimiter(rate.Every(time.Second/3), 1),
		config:    cfg,
		userAgent: _ua,
	}, nil
}

// Service implements core.ArtistGetter.
func (*Client) Service() string {
	return _artistBioService
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

