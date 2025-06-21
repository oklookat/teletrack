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
	_state = "abc123"

	// So far, there have been no problems with AU, for example when searching.
	_market = spotify.Market("AU")
)

func Authorize(ctx context.Context,
	cfg *config.Spotify,
	onURL func(url string)) (*oauth2.Token, error) {

	return getTokens(ctx, cfg.RedirectURI, cfg.ClientID, cfg.ClientSecret, onURL)
}

func getTokens(ctx context.Context,
	redirectURI,
	clientID,
	clientSecret string,
	onURL func(url string)) (*oauth2.Token, error) {

	httpErr := make(chan error)
	auth := getAuthenticator(redirectURI, clientID, clientSecret)

	clientCh := make(chan *spotify.Client)

	go serve(ctx, func(w http.ResponseWriter, r *http.Request) {
		tok, err := auth.Token(r.Context(), _state, r)
		if err != nil {
			httpErr <- err
			return
		}

		if st := r.FormValue("state"); st != _state {
			msg := fmt.Errorf("state mismatch. Actual: %s, got: %s", st, _state)
			w.WriteHeader(404)
			w.Write([]byte(msg.Error()))
			httpErr <- msg
			return
		}

		auClient := auth.Client(r.Context(), tok)

		clientCh <- spotify.New(auClient, spotify.WithRetry(true))
		w.WriteHeader(200)
		w.Write([]byte("Done. Now you can go back to where you came from."))
		httpErr <- err
	}, redirectURI)

	// get auth url
	url := auth.AuthURL(_state)

	// send url to user
	go onURL(url)

	var client *spotify.Client
L:
	for {
		select {
		// check err from http handler
		case err := <-httpErr:
			if err != nil {
				return nil, err
			}
		// check client from handler
		case client = <-clientCh:
			if client == nil {
				return nil, errors.New("nil client")
			}
			break L
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// get tokens
	token, err := client.Token()
	if err != nil {
		return nil, err
	}

	if errors.Is(ctx.Err(), context.Canceled) {
		return nil, context.Canceled
	}

	return token, err
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

func serve(ctx context.Context, what http.HandlerFunc, redirectURI string) (err error) {

	rediURL, err := url.Parse(redirectURI)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle(rediURL.Path, what)

	port := ":" + rediURL.Port()
	srv := &http.Server{
		Addr:    port,
		Handler: mux,
	}

	go func() {
		if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server listen: " + err.Error())
		}
	}()

	slog.Debug("server started")

	<-ctx.Done()

	slog.Debug("server stopped")

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()

	if err = srv.Shutdown(ctxShutDown); err != nil {
		if !(errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			slog.Error("shutdown: " + err.Error())
		}
	}

	if err == http.ErrServerClosed {
		err = nil
	}

	return
}

func GetClient(redirectURI, clientID, clientSecret string, token *oauth2.Token) *spotify.Client {
	au := getAuthenticator(redirectURI, clientID, clientSecret)
	auClient := au.Client(context.Background(), token)
	client := spotify.New(auClient)
	return client
}
