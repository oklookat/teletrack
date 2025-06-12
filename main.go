package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/oklookat/teletrack/config"
	"github.com/oklookat/teletrack/module"
	"github.com/oklookat/teletrack/spotify"
	"github.com/oklookat/teletrack/telegram"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	chk(config.Boot())

	// Spotify.
	if config.C.Spotify.Authorize {
		chk(authorizeSpotify(ctx))
		println("Done.")
		os.Exit(0)
	}
	spotifyCl := spotify.GetClient(config.C.Spotify.RedirectURI, config.C.Spotify.ClientID, config.C.Spotify.ClientSecret, config.C.Spotify.Token)

	// Bot.
	telegram.Boot(ctx, config.C.Telegram, []telegram.Module{
		module.NewSpotifyPlayer(spotifyCl),
	})
	for {
		<-ctx.Done()
		break
	}
}

func chk(err error) {
	if err == nil {
		return
	}
	println(err.Error())
	os.Exit(1)
}

func authorizeSpotify(ctx context.Context) error {
	token, err := spotify.Authorize(ctx, config.C.Spotify, func(url string) {
		println("Go to URL: " + url)
	})
	if err != nil {
		return err
	}
	config.C.Spotify.Authorize = false
	config.C.Spotify.Token = token
	return config.C.Save()
}
