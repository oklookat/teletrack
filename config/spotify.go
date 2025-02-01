package config

import "golang.org/x/oauth2"

type Spotify struct {
	Authorize    bool          `json:"authorize"`
	RedirectURI  string        `json:"redirectURI"`
	ClientID     string        `json:"clientID"`
	ClientSecret string        `json:"clientSecret"`
	Token        *oauth2.Token `json:"token"`
}
