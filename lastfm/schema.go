package lastfm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Image represents an image with a size and URL/text.
type Image struct {
	Size string `json:"size"`
	Text string `json:"#text"`
}

// ArtistShort represents minimal artist info.
type ArtistShort struct {
	URL  string `json:"url"`
	Name string `json:"name"`
	Mbid string `json:"mbid"`
}

// AlbumShort represents minimal album info.
type AlbumShort struct {
	Mbid string `json:"mbid"`
	Text string `json:"#text"`
}

// DateInfo represents date information.
type DateInfo struct {
	Uts  string `json:"uts"`
	Text string `json:"#text"`
}

// RankAttr represents a rank attribute.
type RankAttr struct {
	Rank string `json:"rank"`
}

// ApiError represents an error returned by the Last.fm API.
type ApiError struct {
	Message string `json:"message"`
	Code    int    `json:"error"`
}

// Error implements the error interface for ApiError.
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

// Track represents a music track in Last.fm.
type Track struct {
	Artist     *Artist    `json:"artist"`
	Mbid       string     `json:"mbid"`
	Name       string     `json:"name"`
	Image      []Image    `json:"image"`
	Streamable string     `json:"streamable"`
	Album      AlbumShort `json:"album"`
	URL        string     `json:"url"`
	Attr       struct {
		Nowplaying *bool `json:"nowplaying"`
	} `json:"@attr,omitempty"`
	Loved *bool    `json:"loved"`
	Date  DateInfo `json:"date,omitempty"`
}

// UnmarshalJSON provides robust unmarshaling for Track, converting string booleans.
func (t *Track) UnmarshalJSON(data []byte) error {
	type Alias Track
	aux := &struct {
		Attr struct {
			Nowplaying *string `json:"nowplaying"`
		} `json:"@attr,omitempty"`
		Loved *string `json:"loved"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Convert nowplaying from string to bool
	if aux.Attr.Nowplaying != nil {
		val := strings.ToLower(*aux.Attr.Nowplaying) == "true"
		t.Attr.Nowplaying = &val
	} else {
		t.Attr.Nowplaying = nil
	}

	// Convert loved from string to bool
	if aux.Loved != nil {
		val := *aux.Loved == "1"
		t.Loved = &val
	} else {
		t.Loved = nil
	}

	return nil
}

// UserGetRecentTracksResponse represents the response for recent tracks API.
type UserGetRecentTracksResponse struct {
	Recenttracks struct {
		Track []*Track `json:"track"`
		Attr  struct {
			User       string `json:"user"`
			TotalPages string `json:"totalPages"`
			Page       string `json:"page"`
			PerPage    string `json:"perPage"`
			Total      string `json:"total"`
		} `json:"@attr"`
	} `json:"recenttracks"`
}

// UserGetTopTracksResponse represents the response for top tracks API.
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

// UserGetTopArtistsResponse represents the response for top artists API.
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

// ArtistInfo represents detailed information about an artist.
type ArtistInfo struct {
	Artist struct {
		Name       string  `json:"name"`
		URL        string  `json:"url"`
		Image      []Image `json:"image"`
		Streamable string  `json:"streamable"`
		Ontour     string  `json:"ontour"`
		Stats      struct {
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

// Validate checks if the Artist struct has required fields.
func (a *Artist) Validate() error {
	if a.Name == "" {
		return errors.New("artist name is required")
	}
	if a.URL == "" {
		return errors.New("artist URL is required")
	}
	return nil
}

// Validate checks if the Track struct has required fields.
func (t *Track) Validate() error {
	if t.Name == "" {
		return errors.New("track name is required")
	}
	if t.Artist == nil || t.Artist.Name == "" {
		return errors.New("track artist is required")
	}
	return nil
}
