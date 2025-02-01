package lastfm

import "testing"

func getClient() *Client {
	return NewClient("")
}

func TestUserGetRecentTracks(t *testing.T) {
	tracks, err := getClient().UserGetRecentTracks("", tp(1), nil, nil, tp(true), nil)
	if err != nil {
		t.Fatal(err)
	}
	if tracks != nil {
		if len(tracks.Recenttracks.Track) > 0 {
			tr := tracks.Recenttracks.Track[0]
			t.Logf("artist %s, track: %s, mbid: %s", tr.Artist.Name, tr.Name, tr.Mbid)
		}
	}
}

func TestArtistGetInfo(t *testing.T) {
	info, err := getClient().ArtistGetInfo("the hellp", "en")
	if err != nil {
		t.Fatal(err)
	}
	if info != nil {
		if len(info.Artist.Bio.Summary) > 0 {
			//bioSummary := info.Artist.Bio.Summary
			t.Logf("bio summary: %s", info.BioSummaryWithoutLinks())
		}
	}
}

func tp[T any](what T) *T {
	return &what
}
