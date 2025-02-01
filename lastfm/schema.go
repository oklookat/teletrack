package lastfm

import (
	"encoding/json"
	"fmt"
	"regexp"
)

type ApiError struct {
	Message string `json:"message"`
	Code    int    `json:"error"`
}

func (e ApiError) Error() string {
	return fmt.Sprintf("%s, code: %d", e.Message, e.Code)
}

type Artist struct {
	URL   string `json:"url"`
	Name  string `json:"name"`
	Image []struct {
		Size string `json:"size"`
		Text string `json:"#text"`
	} `json:"image"`
	Mbid string `json:"mbid"`
}

type Track struct {
	Artist *Artist `json:"artist"`
	Mbid   string  `json:"mbid"`
	Name   string  `json:"name"`
	Image  []struct {
		Size string `json:"size"`
		Text string `json:"#text"`
	} `json:"image"`
	Streamable string `json:"streamable"`
	Album      struct {
		Mbid string `json:"mbid"`
		Text string `json:"#text"`
	} `json:"album"`
	URL  string `json:"url"`
	Attr struct {
		Nowplaying *bool `json:"nowplaying"`
	} `json:"@attr,omitempty"`
	Loved *bool `json:"loved"`
	Date  struct {
		Uts  string `json:"uts"`
		Text string `json:"#text"`
	} `json:"date,omitempty"`
}

// Custom unmarshaler for Track
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
		val := (*aux.Attr.Nowplaying == "true")
		t.Attr.Nowplaying = &val
	}

	// Convert loved from string to bool
	if aux.Loved != nil {
		val := (*aux.Loved == "1")
		t.Loved = &val
	}

	return nil
}

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

type ArtistInfo struct {
	Artist struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Image []struct {
			Text string `json:"#text"`
			Size string `json:"size"`
		} `json:"image"`
		Streamable string `json:"streamable"`
		Ontour     string `json:"ontour"`
		Stats      struct {
			Listeners string `json:"listeners"`
			Playcount string `json:"playcount"`
		} `json:"stats"`
		Similar struct {
			Artist []struct {
				Name  string `json:"name"`
				URL   string `json:"url"`
				Image []struct {
					Text string `json:"#text"`
					Size string `json:"size"`
				} `json:"image"`
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

func (a ArtistInfo) BioSummaryWithoutLinks() string {
	return cleanAllLinks(a.Artist.Bio.Summary)
}

func (a ArtistInfo) BioContentWithoutLinks() string {
	return cleanAllLinks(a.Artist.Bio.Content)
}

func cleanAllLinks(text string) string {
	// Регулярное выражение для поиска ссылок
	hrefRe := regexp.MustCompile(`<a\s+href="[^"]+">.*?</a>`)

	// Удаляем все совпадения ссылок
	cleanedText := hrefRe.ReplaceAllString(text, "")

	return cleanedText
}
