package spotify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
)

var (
	_state  = "abc123"
	_market = spotify.Market("AU")
)

func newAuthorizer(cfg *Config) *authorizer {
	return &authorizer{
		cfg: cfg,
	}
}

type authorizer struct {
	cfg *Config
}

func (a authorizer) Authorize(ctx context.Context) (*Token, error) {
	if !a.cfg.Authorize {
		return nil, nil
	}

	token, err := a.getTokens(ctx, func(url string) {
		slog.Info("Open Spotify authorization URL", "url", url)
	})

	if err != nil {
		return nil, err
	}

	a.cfg.Authorize = false
	a.cfg.Token = token

	slog.Info("Spotify authorization completed")

	return token, nil
}

func (a authorizer) getTokens(ctx context.Context, onURL func(string)) (*Token, error) {
	auth := a.getAuthenticator(
		a.cfg.RedirectURI,
		a.cfg.ClientID,
		a.cfg.ClientSecret,
	)

	oauthCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	clientCh := make(chan *spotify.Client, 1)
	errCh := make(chan error, 1)

	go func() {
		err := a.serve(oauthCtx, func(w http.ResponseWriter, r *http.Request) {
			a.handleOAuthCallback(
				auth,
				w,
				r,
				clientCh,
				errCh,
			)
		})

		if err != nil && !errors.Is(err, context.Canceled) {
			errCh <- err
		}
	}()

	onURL(auth.AuthURL(_state))

	select {

	case client := <-clientCh:

		if client == nil {
			return nil, errors.New("received nil Spotify client")
		}

		cancel()

		token, err := client.Token()
		if err != nil {
			return nil, err
		}

		slog.Info("Spotify token received")

		return newToken(token), nil

	case err := <-errCh:
		cancel()
		return nil, err

	case <-ctx.Done():
		cancel()
		return nil, ctx.Err()
	}
}

func (a authorizer) handleOAuthCallback(
	auth *spotifyauth.Authenticator,
	w http.ResponseWriter,
	r *http.Request,
	clientCh chan<- *spotify.Client,
	errCh chan<- error,
) {

	token, err := auth.Token(
		r.Context(),
		_state,
		r,
	)

	if err != nil {
		http.Error(
			w,
			fmt.Sprintf("Failed to get token: %v", err),
			http.StatusInternalServerError,
		)

		errCh <- err
		return
	}

	if state := r.FormValue("state"); state != _state {

		err := fmt.Errorf(
			"state mismatch: expected %s got %s",
			_state,
			state,
		)

		http.Error(
			w,
			err.Error(),
			http.StatusBadRequest,
		)

		errCh <- err
		return
	}

	client := spotify.New(
		auth.Client(r.Context(), token),
		spotify.WithRetry(true),
	)

	// Сначала отвечаем браузеру
	w.Header().Set(
		"Content-Type",
		"text/html; charset=utf-8",
	)

	w.WriteHeader(http.StatusOK)

	fmt.Fprint(w, `
<!DOCTYPE html>
<html>
<head>
<title>Spotify Authorization</title>
</head>
<body>
<h2>Authorization completed</h2>
<p>You can close this window.</p>
</body>
</html>
`)

	slog.Info("Spotify OAuth callback received")

	// Передаем клиент дальше
	clientCh <- client
}

func (a authorizer) getAuthenticator(
	redirectURI,
	clientID,
	clientSecret string,
) *spotifyauth.Authenticator {

	return spotifyauth.New(
		spotifyauth.WithClientID(clientID),
		spotifyauth.WithClientSecret(clientSecret),
		spotifyauth.WithRedirectURL(redirectURI),

		spotifyauth.WithScopes(
			spotifyauth.ScopeUserReadCurrentlyPlaying,
			spotifyauth.ScopeUserReadPrivate,
			spotifyauth.ScopeUserLibraryRead,
			spotifyauth.ScopeUserLibraryModify,
			spotifyauth.ScopeUserFollowRead,
			spotifyauth.ScopeUserFollowModify,
			spotifyauth.ScopePlaylistReadPrivate,
			spotifyauth.ScopePlaylistModifyPrivate,
			spotifyauth.ScopePlaylistModifyPublic,
		),
	)
}

// временный OAuth HTTP сервер
func (a authorizer) serve(
	ctx context.Context,
	handler http.HandlerFunc,
) error {

	u, err := url.Parse(a.cfg.RedirectURI)

	if err != nil {
		return fmt.Errorf(
			"invalid redirect URI: %w",
			err,
		)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(
		u.Path,
		handler,
	)

	srv := &http.Server{
		Addr:    ":" + u.Port(),
		Handler: mux,
	}

	go func() {

		slog.Info(
			"Spotify OAuth server started",
			"port",
			u.Port(),
		)

		err := srv.ListenAndServe()

		if err != nil &&
			!errors.Is(err, http.ErrServerClosed) {

			slog.Error(
				"Spotify OAuth server error",
				"error",
				err,
			)
		}

	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)

	defer cancel()

	err = srv.Shutdown(shutdownCtx)

	if err != nil &&
		!errors.Is(err, context.Canceled) {

		slog.Error(
			"Spotify OAuth shutdown error",
			"error",
			err,
		)

		return err
	}

	slog.Info("Spotify OAuth server stopped")

	return nil
}

func (a authorizer) getClient(
	redirectURI,
	clientID,
	clientSecret string,
	token *Token,
) *spotify.Client {

	auth := a.getAuthenticator(
		redirectURI,
		clientID,
		clientSecret,
	)

	httpClient := auth.Client(
		context.Background(),
		token.oauth2(),
	)

	return spotify.New(
		httpClient,
		spotify.WithRetry(true),
	)
}
