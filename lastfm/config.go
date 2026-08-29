package lastfm

type Config struct {
	APIKey   string `json:"apiKey"`
	Username string `json:"username"`
}

func (c Config) Validate() bool {
	return c.APIKey != "" && c.Username != ""
}
