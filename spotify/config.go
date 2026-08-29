package spotify

import (
	"time"

	"golang.org/x/oauth2"
)

type Config struct {
	Authorize    bool   `json:"authorize"`
	RedirectURI  string `json:"redirectURI"`
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
	Token        *Token `json:"token"`
}

func (c Config) Validate() bool {
	return c.RedirectURI != "" && c.ClientID != "" && c.ClientSecret != "" && c.Token != nil && c.Token.Validate()
}

type Token struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	ExpiresIn    int64     `json:"expires_in"`
}

func (t Token) Validate() bool {
	return t.AccessToken != "" && t.RefreshToken != ""
}

func newToken(from *oauth2.Token) *Token {
	if from == nil {
		return nil
	}
	return &Token{
		AccessToken:  from.AccessToken,
		TokenType:    from.TokenType,
		RefreshToken: from.RefreshToken,
		Expiry:       from.Expiry,
		ExpiresIn:    from.ExpiresIn,
	}
}

func (t Token) oauth2() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  t.AccessToken,
		TokenType:    t.TokenType,
		RefreshToken: t.RefreshToken,
		Expiry:       t.Expiry,
		ExpiresIn:    t.ExpiresIn,
	}
}
