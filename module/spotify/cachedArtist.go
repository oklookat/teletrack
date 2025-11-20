package spotify

import (
	"fmt"
	"time"

	"github.com/oklookat/teletrack/lastfm"
	"github.com/oklookat/teletrack/shared"
	"github.com/oklookat/teletrack/shared/lastfmclean"
)

const (
	artistInfoFetchTimeout = 5 * time.Second
)

type cachedArtistInfo struct {
	// TG-formatted
	Bio        string
	LastFmLink string
}

func (a *cachedArtistInfo) format(info *lastfm.ArtistInfo) {
	if info != nil {
		a.setBio(info)
		if len(info.Artist.URL) > 0 {
			a.LastFmLink = fmt.Sprintf("🔗 %s", shared.TgLink("Last.fm", info.Artist.URL))
		}
	}
}

func (a *cachedArtistInfo) setBio(info *lastfm.ArtistInfo) {
	if info == nil {
		return
	}

	cleaner := lastfmclean.NewCleaner(lastfmclean.Config{
		MaxLength:        300,
		RemoveHTML:       true,
		RemoveReferences: true,
		RemoveReadMore:   true,
		ExtractFirstOnly: true,
		RemoveMarkdown:   true,
	})

	bio := cleaner.Clean(info.Artist.Bio.Summary)
	a.Bio = shared.TgText(bio)
}
