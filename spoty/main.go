package spoty

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/oklookat/teletrack/config"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

var (
	_state  = "abc123"
	_market = spotify.Market("AU")
)

// Authorize initiates the Spotify OAuth flow and returns a token.
func Authorize(ctx context.Context, cfg *config.Spotify, onURL func(string)) (*oauth2.Token, error) {
	return getTokens(ctx, cfg.RedirectURI, cfg.ClientID, cfg.ClientSecret, onURL)
}

func getTokens(ctx context.Context, redirectURI, clientID, clientSecret string, onURL func(string)) (*oauth2.Token, error) {
	auth := getAuthenticator(redirectURI, clientID, clientSecret)
	clientCh := make(chan *spotify.Client, 1)
	errCh := make(chan error, 1)

	// Start temporary HTTP server to handle redirect
	go func() {
		if err := serve(ctx, redirectURI, func(w http.ResponseWriter, r *http.Request) {
			handleOAuthCallback(auth, w, r, clientCh, errCh)
		}); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- err
		}
	}()

	// Provide URL to the user
	go onURL(auth.AuthURL(_state))

	select {
	case client := <-clientCh:
		if client == nil {
			return nil, errors.New("received nil Spotify client")
		}
		return client.Token()
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func handleOAuthCallback(auth *spotifyauth.Authenticator, w http.ResponseWriter, r *http.Request, clientCh chan<- *spotify.Client, errCh chan<- error) {
	token, err := auth.Token(r.Context(), _state, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get token: %v", err), http.StatusInternalServerError)
		errCh <- err
		return
	}

	if state := r.FormValue("state"); state != _state {
		msg := fmt.Errorf("state mismatch. Expected %s, got %s", _state, state)
		http.Error(w, msg.Error(), http.StatusBadRequest)
		errCh <- msg
		return
	}

	client := spotify.New(auth.Client(r.Context(), token), spotify.WithRetry(true))
	clientCh <- client

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Authorization successful! You can now return to the application."))
}

func getAuthenticator(redirectURI, clientID, clientSecret string) *spotifyauth.Authenticator {
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

// serve runs a temporary HTTP server to handle OAuth redirect
func serve(ctx context.Context, redirectURI string, handler http.HandlerFunc) error {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return fmt.Errorf("invalid redirect URI: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(u.Path, handler)

	port := ":" + u.Port()
	srv := &http.Server{
		Addr:    port,
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server listen error: " + err.Error())
		}
	}()

	slog.Debug("Spotify OAuth server started on port " + u.Port())

	<-ctx.Done()

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctxShutDown); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("server shutdown error: " + err.Error())
	}

	slog.Debug("Spotify OAuth server stopped")
	return nil
}

// GetClient returns a Spotify client from an OAuth token
func GetClient(redirectURI, clientID, clientSecret string, token *oauth2.Token) *spotify.Client {
	auth := getAuthenticator(redirectURI, clientID, clientSecret)
	httpClient := auth.Client(context.Background(), token)
	return spotify.New(httpClient, spotify.WithRetry(true))
}
