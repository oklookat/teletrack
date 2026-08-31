package spotify

import (
	"log/slog"
	"sync"

	"golang.org/x/oauth2"
)

// persistingTokenSource wraps an oauth2.TokenSource and calls save whenever
// the access token changes (typically after a refresh). Failures to persist
// are logged but do not fail the Token() call — API traffic should continue.
type persistingTokenSource struct {
	base       oauth2.TokenSource
	save       func(*Token) error
	mu         sync.Mutex
	lastAccess string
}

func (s *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := s.base.Token()
	if err != nil {
		return nil, err
	}
	if tok == nil {
		return nil, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if tok.AccessToken == s.lastAccess || s.save == nil {
		return tok, nil
	}
	s.lastAccess = tok.AccessToken

	if err := s.save(newToken(tok)); err != nil {
		slog.Error("failed to persist refreshed Spotify token", "error", err)
		// Still return the token so playback keeps working.
	} else {
		slog.Info("persisted refreshed Spotify token")
	}
	return tok, nil
}
