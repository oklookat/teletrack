package toolbox

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func RandomMemeUrl() (string, error) {
	const end = "https://meme-api.com/gimme"

	resp, err := http.Get(end)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	type respBody struct {
		PostLink  string   `json:"postLink"`
		Subreddit string   `json:"subreddit"`
		Title     string   `json:"title"`
		URL       string   `json:"url"`
		Nsfw      bool     `json:"nsfw"`
		Spoiler   bool     `json:"spoiler"`
		Author    string   `json:"author"`
		Ups       int      `json:"ups"`
		Preview   []string `json:"preview"`
	}
	var bodyDec respBody
	if err := json.NewDecoder(resp.Body).Decode(&bodyDec); err != nil {
		return "", err
	}

	return bodyDec.URL, err
}
