// Package listenbrainz implements core.Player and core.ArtistGetter using
// ListenBrainz (now-playing / recent listens) plus MusicBrainz and Wikidata
// for artist biographies.
package listenbrainz

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/oklookat/teletrack/core"
)

// lbAdditionalInfo holds optional client-submitted identifiers that may be
// present on a listen even when the server has not yet produced mbid_mapping.
type lbAdditionalInfo struct {
	RecordingMBID string   `json:"recording_mbid"`
	ReleaseMBID   string   `json:"release_mbid"`
	ArtistMBIDs   []string `json:"artist_mbids"`
	DurationMS    int64    `json:"duration_ms"`
	SpotifyID     string   `json:"spotify_id"`
	OriginURL     string   `json:"origin_url"`
	TrackNumber   int      `json:"tracknumber"`
}

type lbMBIDMapping struct {
	RecordingMBID  string `json:"recording_mbid"`
	ReleaseMBID    string `json:"release_mbid"`
	CAAReleaseMBID string `json:"caa_release_mbid"`
	CAAID          int64  `json:"caa_id"`
	RecordingName  string `json:"recording_name"`
}

type lbListen struct {
	ListenedAt    int64  `json:"listened_at"`
	PlayingNow    bool   `json:"playing_now"`
	RecordingMSID string `json:"recording_msid"`
	TrackMetadata struct {
		ArtistName     string            `json:"artist_name"`
		TrackName      string            `json:"track_name"`
		ReleaseName    string            `json:"release_name"`
		AdditionalInfo *lbAdditionalInfo `json:"additional_info"`
		MBIDMapping    *lbMBIDMapping    `json:"mbid_mapping"`
	} `json:"track_metadata"`
}

type lbListensResponse struct {
	Payload struct {
		Count      int        `json:"count"`
		PlayingNow bool       `json:"playing_now"`
		Listens    []lbListen `json:"listens"`
	} `json:"payload"`
}

// lbMetadataLookup is the response from GET /1/metadata/lookup/.
type lbMetadataLookup struct {
	RecordingMBID string `json:"recording_mbid"`
	ReleaseMBID   string `json:"release_mbid"`
	RecordingName string `json:"recording_name"`
	ReleaseName   string `json:"release_name"`
}

// GetPlaying returns the user's currently playing track, or the most recent
// listen if nothing is playing right now.
// If consecutive server errors occur, it returns nil without error.
func (c *Client) GetPlaying(ctx context.Context) (core.Track, error) {
	// 1. Prefer the playing-now endpoint.
	var resp lbListensResponse
	path := "user/" + url.PathEscape(c.config.Username) + "/playing-now"
	err := c.getJSON(ctx, c.lbLimiter, _lbAPIURL, path, nil, &resp, true)
	if err != nil {
		if errors.Is(err, ErrRepeatedServerError) {
			return nil, nil
		}
		return nil, fmt.Errorf("playing-now: %w", err)
	}
	if len(resp.Payload.Listens) > 0 {
		return c.trackInfoFromListen(ctx, resp.Payload.Listens[0]), nil
	}

	// 2. Nothing is playing — fall back to the latest listen.
	q := url.Values{"count": {"1"}}
	path = "user/" + url.PathEscape(c.config.Username) + "/listens"
	err = c.getJSON(ctx, c.lbLimiter, _lbAPIURL, path, q, &resp, true)
	if err != nil {
		if errors.Is(err, ErrRepeatedServerError) {
			return nil, nil
		}
		return nil, fmt.Errorf("listens: %w", err)
	}
	if len(resp.Payload.Listens) == 0 {
		return nil, nil
	}
	return c.trackInfoFromListen(ctx, resp.Payload.Listens[0]), nil
}

func (c *Client) trackInfoFromListen(ctx context.Context, l lbListen) *TrackInfo {
	trackLink, cover := coverAndLinkFromListen(l)

	// Playing-now payloads often lack mbid_mapping. Resolve cover via
	// ListenBrainz metadata lookup (token) or MusicBrainz search.
	if cover == "" {
		artist := strings.TrimSpace(l.TrackMetadata.ArtistName)
		track := strings.TrimSpace(l.TrackMetadata.TrackName)
		release := strings.TrimSpace(l.TrackMetadata.ReleaseName)
		if artist != "" && track != "" {
			if mbid, releaseMBID := c.resolveReleaseMBID(ctx, artist, track, release); releaseMBID != "" {
				cover = coverArtURL(releaseMBID, 0)
				if trackLink == "" && mbid != "" {
					trackLink = "https://musicbrainz.org/recording/" + mbid
				}
			}
		}
	}

	when := time.Now()
	if !l.PlayingNow && l.ListenedAt > 0 {
		when = time.Unix(l.ListenedAt, 0)
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

// coverAndLinkFromListen extracts cover URL and track link from fields already
// present on the listen (mbid_mapping and additional_info). No network calls.
func coverAndLinkFromListen(l lbListen) (trackLink, cover string) {
	if m := l.TrackMetadata.MBIDMapping; m != nil {
		if m.RecordingMBID != "" {
			trackLink = "https://musicbrainz.org/recording/" + m.RecordingMBID
		}
		releaseMBID := m.CAAReleaseMBID
		if releaseMBID == "" {
			releaseMBID = m.ReleaseMBID
		}
		if releaseMBID != "" {
			cover = coverArtURL(releaseMBID, m.CAAID)
		}
	}

	if info := l.TrackMetadata.AdditionalInfo; info != nil {
		if trackLink == "" && info.RecordingMBID != "" {
			trackLink = "https://musicbrainz.org/recording/" + info.RecordingMBID
		}
		if cover == "" && info.ReleaseMBID != "" {
			cover = coverArtURL(info.ReleaseMBID, 0)
		}
		// Prefer a direct streaming URL when we have no MusicBrainz link.
		if trackLink == "" {
			if info.OriginURL != "" {
				trackLink = info.OriginURL
			} else if info.SpotifyID != "" {
				trackLink = info.SpotifyID
			}
		}
	}
	return trackLink, cover
}

// coverArtURL builds a Cover Art Archive front-cover URL.
// When caaID is non-zero the specific image is requested; otherwise the
// generic /front-500 redirect is used.
func coverArtURL(releaseMBID string, caaID int64) string {
	if releaseMBID == "" {
		return ""
	}
	if caaID > 0 {
		return fmt.Sprintf("https://coverartarchive.org/release/%s/%d-500.jpg", releaseMBID, caaID)
	}
	return "https://coverartarchive.org/release/" + releaseMBID + "/front-500"
}

// resolveReleaseMBID tries ListenBrainz metadata lookup (with token when
// available), then falls back to a MusicBrainz recording search.
func (c *Client) resolveReleaseMBID(ctx context.Context, artist, track, release string) (recordingMBID, releaseMBID string) {
	if rec, rel := c.lbMetadataLookup(ctx, artist, track, release); rel != "" || rec != "" {
		return rec, rel
	}
	return c.mbSearchRecording(ctx, artist, track, release)
}

// lbMetadataLookup calls GET /1/metadata/lookup/. Requires a valid token.
func (c *Client) lbMetadataLookup(ctx context.Context, artist, track, release string) (recordingMBID, releaseMBID string) {
	if c.config.Token == "" {
		return "", ""
	}
	q := url.Values{
		"artist_name":    {artist},
		"recording_name": {track},
	}
	if release != "" {
		q.Set("release_name", release)
	}
	var out lbMetadataLookup
	if err := c.getJSON(ctx, c.lbLimiter, _lbAPIURL, "metadata/lookup/", q, &out, true); err != nil {
		return "", ""
	}
	return out.RecordingMBID, out.ReleaseMBID
}

// mbRecordingSearchResponse is a minimal MusicBrainz recording search result.
type mbRecordingSearchResponse struct {
	Recordings []struct {
		ID       string `json:"id"`
		Releases []struct {
			ID string `json:"id"`
		} `json:"releases"`
	} `json:"recordings"`
}

// mbSearchRecording finds a recording (+ first release) via MusicBrainz search.
func (c *Client) mbSearchRecording(ctx context.Context, artist, track, release string) (recordingMBID, releaseMBID string) {
	parts := []string{
		fmt.Sprintf(`recording:%q`, track),
		fmt.Sprintf(`artist:%q`, artist),
	}
	if release != "" {
		parts = append(parts, fmt.Sprintf(`release:%q`, release))
	}
	q := url.Values{
		"query": {strings.Join(parts, " AND ")},
		"fmt":   {"json"},
		"limit": {"1"},
	}
	var resp mbRecordingSearchResponse
	if err := c.getJSON(ctx, c.mbLimiter, _mbAPIURL, "recording/", q, &resp, false); err != nil {
		return "", ""
	}
	if len(resp.Recordings) == 0 {
		return "", ""
	}
	rec := resp.Recordings[0]
	recordingMBID = rec.ID
	if len(rec.Releases) > 0 {
		releaseMBID = rec.Releases[0].ID
	}
	return recordingMBID, releaseMBID
}
