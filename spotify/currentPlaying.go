package spotify

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
	ProgressMs int
	DurationMs int
	Link       string
	CoverURL   *string
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

	var coverURL string
	if len(curPlay.Item.Album.Images) > 0 {
		coverURL = curPlay.Item.Album.Images[0].URL
	}

	sTrack := curPlay.Item.SimpleTrack

	var artistsNames []string
	for _, ar := range sTrack.Artists {
		artistsNames = append(artistsNames, ar.Name)
	}

	curPlaying := &CurrentPlaying{
		ID:         sTrack.ID.String(),
		Name:       sTrack.Name,
		Artists:    strings.Join(artistsNames, ", "),
		DurationMs: int(sTrack.Duration),
		ProgressMs: int(curPlay.Progress),
		Link:       spotifyLink,
		CoverURL:   &coverURL,
	}
	if len(artistsNames) > 0 {
		curPlaying.Artist = artistsNames[0]
	}

	return curPlaying, err
}
