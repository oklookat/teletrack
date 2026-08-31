package spotify

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type staticSource struct {
	tok *oauth2.Token
	err error
}

func (s staticSource) Token() (*oauth2.Token, error) {
	return s.tok, s.err
}

func TestPersistingTokenSource_SavesOnChange(t *testing.T) {
	var saves int32
	src := &persistingTokenSource{
		base: staticSource{tok: &oauth2.Token{
			AccessToken:  "new-access",
			RefreshToken: "ref",
			Expiry:       time.Now().Add(time.Hour),
		}},
		lastAccess: "old-access",
		save: func(tok *Token) error {
			atomic.AddInt32(&saves, 1)
			if tok.AccessToken != "new-access" {
				t.Errorf("unexpected token %q", tok.AccessToken)
			}
			return nil
		},
	}
	tok, err := src.Token()
	if err != nil || tok.AccessToken != "new-access" {
		t.Fatalf("Token: %v %#v", err, tok)
	}
	if atomic.LoadInt32(&saves) != 1 {
		t.Fatalf("saves = %d", saves)
	}

	// Second call with same access token must not save again.
	_, _ = src.Token()
	if atomic.LoadInt32(&saves) != 1 {
		t.Fatalf("saves after second call = %d", saves)
	}
}

func TestPersistingTokenSource_SaveErrorDoesNotFailToken(t *testing.T) {
	src := &persistingTokenSource{
		base: staticSource{tok: &oauth2.Token{AccessToken: "a", RefreshToken: "r"}},
		save: func(*Token) error { return errors.New("disk full") },
	}
	tok, err := src.Token()
	if err != nil {
		t.Fatalf("Token should succeed despite save error: %v", err)
	}
	if tok.AccessToken != "a" {
		t.Fatal(tok)
	}
}
