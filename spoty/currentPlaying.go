package spoty

import (
	"context"
	"strings"

	"github.com/zmb3/spotify/v2"
)

type CurrentPlaying struct {
	ID         string
	Name       string
	Artists    string
	Artist     string
	ArtistID   string
	ProgressMs int
	DurationMs int
	Link       string
	CoverURL   *string
	Playing    bool

	FullTrack *spotify.FullTrack
}

func GetCurrentPlaying(ctx context.Context, cl *spotify.Client) (*CurrentPlaying, error) {
	curPlay, err := cl.PlayerCurrentlyPlaying(ctx, _market)
	if err != nil {
		return nil, err
	}

	if curPlay == nil || curPlay.Item == nil || curPlay.Item.ExternalURLs == nil {
		return nil, nil
	}

	spotifyLink, ok := curPlay.Item.ExternalURLs["spotify"]
	if !ok {
		return nil, nil
	}

	var coverURL *string
	if len(curPlay.Item.Album.Images) > 0 {
		coverURL = &curPlay.Item.Album.Images[0].URL
	}

	sTrack := curPlay.Item.SimpleTrack

	var artistID string
	var artistsNames []string
	for _, ar := range sTrack.Artists {
		if len(artistID) == 0 {
			artistID = ar.ID.String()
		}
		artistsNames = append(artistsNames, ar.Name)
	}

	trackID := sTrack.ID.String()
	if len(artistID) == 0 || len(artistsNames) == 0 || len(trackID) == 0 {
		return nil, nil
	}

	curPlaying := &CurrentPlaying{
		ID:         sTrack.ID.String(),
		Name:       sTrack.Name,
		Artists:    strings.Join(artistsNames, ", "),
		Artist:     artistsNames[0],
		ArtistID:   artistID,
		DurationMs: int(sTrack.Duration),
		ProgressMs: int(curPlay.Progress),
		Link:       spotifyLink,
		CoverURL:   coverURL,
		Playing:    curPlay.Playing,
		FullTrack:  curPlay.Item,
	}

	return curPlaying, nil
}
