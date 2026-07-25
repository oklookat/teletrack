package lastfm

import "testing"

func getClient(t *testing.T) *Client {
	cl, err := NewClient(&Config{})
	if err != nil {
		t.Fatal(err)
	}
	return cl
}

// func TestUserGetRecentTracks(t *testing.T) {
// 	tracks, err := getClient(t).UserGetRecentTracks(new(1), nil, nil, new(true), nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if tracks != nil {
// 		if len(tracks.Recenttracks.Track) > 0 {
// 			tr := tracks.Recenttracks.Track[0]
// 			t.Logf("artist %s, track: %s, mbid: %s", tr.Artist.Name, tr.Name, tr.Mbid)
// 		}
// 	}
// }
