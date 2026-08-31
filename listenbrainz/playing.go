// Package listenbrainz implements core.Player and core.ArtistGetter using
// ListenBrainz (now-playing / recent listens) plus MusicBrainz and Wikidata
// for artist biographies.
package listenbrainz

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/oklookat/teletrack/core"
)

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
func (c *Client) GetPlaying(ctx context.Context) (core.Track, error) {
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

