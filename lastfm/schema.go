package lastfm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Image represents an image with a size and URL/text.
type Image struct {
	Size string `json:"size"`
	Text string `json:"#text"`
}

// ArtistShort represents minimal artist information.
type ArtistShort struct {
	URL  string `json:"url"`
	Name string `json:"name"`
	Mbid string `json:"mbid"`
}

// AlbumShort represents minimal album information.
type AlbumShort struct {
	Mbid string `json:"mbid"`
	Text string `json:"#text"`
}

// DateInfo represents date information returned by Last.fm.
type DateInfo struct {
	Uts  string `json:"uts"`
	Text string `json:"#text"`
}

// ToTime converts Last.fm date information to time.Time safely.
func (d *DateInfo) ToTime() *time.Time {
	if d == nil {
		return nil
	}

	// Try to parse Text first.
	if text := strings.TrimSpace(d.Text); text != "" {
		if t, err := time.Parse("02 Jan 2006, 15:04", text); err == nil {
			return &t
		}
	}

	// Fall back to Unix timestamp.
	if uts := strings.TrimSpace(d.Uts); uts != "" {
		if timestamp, err := strconv.ParseInt(uts, 10, 64); err == nil {
			t := time.Unix(timestamp, 0)
			return &t
		}
	}

	return nil
}

// RankAttr represents a rank attribute returned by Last.fm.
type RankAttr struct {
	Rank string `json:"rank"`
}

// TrackAttr represents the @attr object of a Last.fm track.
type TrackAttr struct {
	Nowplaying *bool `json:"nowplaying,omitempty"`
}

type LastFmBool bool

// UnmarshalJSON implements json.Unmarshaler.
func (b *LastFmBool) UnmarshalJSON(data []byte) error {
	if b == nil {
		return errors.New("LastFmBool target is nil pointer")
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil
	}

	// JSON boolean.
	var boolValue bool
	if err := json.Unmarshal([]byte(trimmed), &boolValue); err == nil {
		*b = LastFmBool(boolValue)
		return nil
	}

	// JSON string.
	var stringValue string
	if err := json.Unmarshal([]byte(trimmed), &stringValue); err == nil {
		switch strings.ToLower(strings.TrimSpace(stringValue)) {
		case "true", "1":
			*b = true
			return nil
		case "false", "0":
			*b = false
			return nil
		default:
			return fmt.Errorf("invalid Last.fm boolean string %q", stringValue)
		}
	}

	// JSON number.
	var numberValue int
	if err := json.Unmarshal([]byte(trimmed), &numberValue); err == nil {
		switch numberValue {
		case 1:
			*b = true
			return nil
		case 0:
			*b = false
			return nil
		default:
			return fmt.Errorf("invalid Last.fm boolean number %d", numberValue)
		}
	}

	return fmt.Errorf("invalid Last.fm boolean: %s", data)
}

// MarshalJSON implements json.Marshaler.
func (b LastFmBool) MarshalJSON() ([]byte, error) {
	return json.Marshal(bool(b))
}

// ApiError represents an error returned by the Last.fm API.
type ApiError struct {
	Message string `json:"message"`
	Code    int    `json:"error"`
}

// Error implements the error interface for ApiError safely.
func (e ApiError) Error() string {
	return fmt.Sprintf("%s, code: %d", e.Message, e.Code)
}

// Artist represents an artist in Last.fm.
type Artist struct {
	URL   string  `json:"url"`
	Name  string  `json:"name"`
	Image []Image `json:"image"`
	Mbid  string  `json:"mbid"`
}

// Validate checks whether the Artist contains required fields.
func (a *Artist) Validate() error {
	if a == nil {
		return errors.New("artist is nil")
	}

	if strings.TrimSpace(a.Name) == "" {
		return errors.New("artist name is required")
	}

	if strings.TrimSpace(a.URL) == "" {
		return errors.New("artist URL is required")
	}

	return nil
}

// Track represents a music track returned by Last.fm.
type Track struct {
	Artist     *Artist     `json:"artist"`
	Mbid       string      `json:"mbid"`
	Name       string      `json:"name"`
	Image      []Image     `json:"image"`
	Streamable string      `json:"streamable"`
	Album      *AlbumShort `json:"album,omitempty"`
	URL        string      `json:"url"`
	Attr       *TrackAttr  `json:"@attr,omitempty"`
	Loved      *LastFmBool `json:"loved,omitempty"`
	Date       *DateInfo   `json:"date,omitempty"`
}

// IsLoved returns whether the track is marked as loved safely.
func (t *Track) IsLoved() bool {
	if t == nil || t.Loved == nil {
		return false
	}

	return bool(*t.Loved)
}

// IsNowPlaying returns whether the track is currently playing safely.
func (t *Track) IsNowPlaying() bool {
	if t == nil || t.Attr == nil || t.Attr.Nowplaying == nil {
		return false
	}

	return *t.Attr.Nowplaying
}

// Validate checks whether the Track contains required fields.
func (t *Track) Validate() error {
	if t == nil {
		return errors.New("track is nil")
	}

	if strings.TrimSpace(t.Name) == "" {
		return errors.New("track name is required")
	}

	if t.Artist == nil {
		return errors.New("track artist is required")
	}

	if err := t.Artist.Validate(); err != nil {
		return fmt.Errorf("invalid track artist: %w", err)
	}

	return nil
}

// UserGetRecentTracksResponse represents the response from user.getrecenttracks.
type UserGetRecentTracksResponse struct {
	Recenttracks struct {
		Track []*Track `json:"track"`

		Attr struct {
			User       string `json:"user"`
			TotalPages string `json:"totalPages"`
			Page       string `json:"page"`
			PerPage    string `json:"perPage"`
			Total      string `json:"total"`
		} `json:"@attr"`
	} `json:"recenttracks"`
}

// UserGetTopTracksResponse represents the response from user.gettoptracks.
type UserGetTopTracksResponse struct {
	Toptracks struct {
		Track []struct {
			Streamable struct {
				Fulltrack string `json:"fulltrack"`
				Text      string `json:"#text"`
			} `json:"streamable"`

			Mbid      string      `json:"mbid"`
			Name      string      `json:"name"`
			Image     []Image     `json:"image"`
			Artist    ArtistShort `json:"artist"`
			URL       string      `json:"url"`
			Duration  string      `json:"duration"`
			Attr      RankAttr    `json:"@attr"`
			Playcount string      `json:"playcount"`
		} `json:"track"`

		Attr struct {
			User       string `json:"user"`
			TotalPages string `json:"totalPages"`
			Page       string `json:"page"`
			PerPage    string `json:"perPage"`
			Total      string `json:"total"`
		} `json:"@attr"`
	} `json:"toptracks"`
}

// UserGetTopArtistsResponse represents the response from user.gettopartists.
type UserGetTopArtistsResponse struct {
	Topartists struct {
		Artist []struct {
			Streamable string   `json:"streamable"`
			Image      []Image  `json:"image"`
			Mbid       string   `json:"mbid"`
			URL        string   `json:"url"`
			Playcount  string   `json:"playcount"`
			Attr       RankAttr `json:"@attr"`
			Name       string   `json:"name"`
		} `json:"artist"`

		Attr struct {
			User       string `json:"user"`
			TotalPages string `json:"totalPages"`
			Page       string `json:"page"`
			PerPage    string `json:"perPage"`
			Total      string `json:"total"`
		} `json:"@attr"`
	} `json:"topartists"`
}

// ArtistFull represents detailed information about an artist.
type ArtistFull struct {
	Artist struct {
		Name       string  `json:"name"`
		URL        string  `json:"url"`
		Image      []Image `json:"image"`
		Streamable string  `json:"streamable"`
		Ontour     string  `json:"ontour"`

		Stats struct {
			Listeners string `json:"listeners"`
			Playcount string `json:"playcount"`
		} `json:"stats"`

		Similar struct {
			Artist []struct {
				Name  string  `json:"name"`
				URL   string  `json:"url"`
				Image []Image `json:"image"`
			} `json:"artist"`
		} `json:"similar"`

		Tags struct {
			Tag []struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			} `json:"tag"`
		} `json:"tags"`

		Bio struct {
			Links struct {
				Link struct {
					Text string `json:"#text"`
					Rel  string `json:"rel"`
					Href string `json:"href"`
				} `json:"link"`
			} `json:"links"`

			Published string `json:"published"`
			Summary   string `json:"summary"`
			Content   string `json:"content"`
		} `json:"bio"`
	} `json:"artist"`
}
