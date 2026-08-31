package listenbrainz

import (
	"context"
	"testing"
)

// todo get from env
// User token from https://listenbrainz.org/settings
const _token = ""

func TestMain(t *testing.T) {
	cl := newClient(t)

	ctx := context.Background()

	aInfo, err := cl.GetArtistInfo(ctx, "Gap Girls", []string{"ru"})
	chk(t, err)
	t.Log(aInfo)

}

func TestGetPlaying(t *testing.T) {
	cl := newClient(t)
	ctx := context.Background()
	pl, err := cl.GetPlaying(ctx)
	chk(t, err)
	t.Log(pl)

}

func newClient(t *testing.T) *Client {
	cl, err := NewClient(&Config{
		Username: "oklookat",
		Token:    _token,
	})
	chk(t, err)
	return cl
}

func chk(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
}
