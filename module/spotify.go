package module

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/oklookat/teletrack/config"
	"github.com/oklookat/teletrack/lastfm"
	"github.com/oklookat/teletrack/shared"
	"github.com/oklookat/teletrack/spoty"

	spotifyapi "github.com/zmb3/spotify/v2"
)

const (
	spotifyRateLimitSec     = 4
	spotifyRateLimit        = spotifyRateLimitSec * time.Second
	spotifyLastProgressIdle = 3 * (spotifyRateLimitSec / 2)
	bioCacheTTL             = 5 * time.Minute
	artistInfoFetchTimeout  = 5 * time.Second
)

// SpotifyPlayerHooks defines interface for player event hooks
type SpotifyPlayerHooks interface {
	OnNothingPlaying(ctx context.Context, b *bot.Bot)
	OnNewTrackPlayed(ctx context.Context, b *bot.Bot, track *spoty.CurrentPlaying)
	OnOldTrackStillPlaying(ctx context.Context, b *bot.Bot, track *spoty.CurrentPlaying)
}

// SpotifyPlayer monitors Spotify playback
type SpotifyPlayer struct {
	client   *spotifyapi.Client
	hooks    SpotifyPlayerHooks
	onError  func(error) error
	shutdown chan struct{}
	wg       sync.WaitGroup

	sync.RWMutex
	lastPlayed       *spoty.CurrentPlaying
	lastProgressTime time.Time
}

// NewSpotifyPlayer creates a new Spotify player instance
func NewSpotifyPlayer(client *spotifyapi.Client, onError func(error) error) *SpotifyPlayer {
	player := &SpotifyPlayer{
		client:   client,
		onError:  onError,
		shutdown: make(chan struct{}),
	}
	player.hooks = newSpotifyPlayerHookImpl(lastfm.NewClient(config.C.LastFm.APIKey), onError, player.shutdown)
	return player
}

// Handle starts the Spotify player monitoring
func (p *SpotifyPlayer) Handle(ctx context.Context, b *bot.Bot) {
	p.wg.Add(1)
	go p.monitorLoop(ctx, b)
}

// Shutdown gracefully stops monitoring
func (p *SpotifyPlayer) Shutdown() {
	close(p.shutdown)
	p.wg.Wait()
}

func (p *SpotifyPlayer) monitorLoop(ctx context.Context, b *bot.Bot) {
	defer p.wg.Done()
	ticker := time.NewTicker(spotifyRateLimit)
	defer ticker.Stop()

	for {
		select {
		case <-p.shutdown:
			return
		case <-ctx.Done():
			if p.onError != nil {
				p.onError(ctx.Err())
			}
			return
		case <-ticker.C:
			if err := p.handleTick(ctx, b); err != nil && p.onError != nil {
				p.onError(err)
			}
		}
	}
}

func (p *SpotifyPlayer) handleTick(ctx context.Context, b *bot.Bot) error {
	currentPlaying, err := spoty.GetCurrentPlaying(ctx, p.client)
	if err != nil {
		return fmt.Errorf("get current playing: %w", err)
	}
	if p.hooks == nil {
		return nil
	}

	now := time.Now()
	p.Lock()
	defer p.Unlock()

	if currentPlaying == nil {
		p.hooks.OnNothingPlaying(ctx, b)
		p.lastPlayed = nil
		p.lastProgressTime = time.Time{}
		return nil
	}

	if p.lastPlayed != nil && currentPlaying.ID == p.lastPlayed.ID && !currentPlaying.Playing {
		if !p.lastProgressTime.IsZero() && now.Sub(p.lastProgressTime) > spotifyLastProgressIdle {
			p.hooks.OnNothingPlaying(ctx, b)
			return nil
		}
		p.hooks.OnOldTrackStillPlaying(ctx, b, currentPlaying)
		return nil
	}

	p.lastProgressTime = now
	if p.lastPlayed == nil || p.lastPlayed.ID != currentPlaying.ID {
		p.hooks.OnNewTrackPlayed(ctx, b, currentPlaying)
	} else {
		p.hooks.OnOldTrackStillPlaying(ctx, b, currentPlaying)
	}
	p.lastPlayed = currentPlaying
	return nil
}

// ---------------------- Spotify Player Hooks Implementation ----------------------

type cachedBio struct {
	info          *lastfm.ArtistInfo
	expiresAt     time.Time
	cachedMessage string
}

type spotifyPlayerHookImpl struct {
	lastFmClient *lastfm.Client
	onError      func(error) error

	mu       sync.RWMutex
	bioCache map[string]*cachedBio
	shutdown <-chan struct{}
}

func newSpotifyPlayerHookImpl(lastFmClient *lastfm.Client, onError func(error) error, shutdown <-chan struct{}) *spotifyPlayerHookImpl {
	h := &spotifyPlayerHookImpl{
		lastFmClient: lastFmClient,
		onError:      onError,
		bioCache:     make(map[string]*cachedBio),
		shutdown:     shutdown,
	}
	go h.backgroundCacheCleaner()
	return h
}

// ---------------------- Event Hooks ----------------------

func (s *spotifyPlayerHookImpl) OnNothingPlaying(ctx context.Context, b *bot.Bot) {
	msg := s.buildIdleMessage()
	s.sendToBot(ctx, b, nil, msg)
}

func (s *spotifyPlayerHookImpl) OnNewTrackPlayed(ctx context.Context, b *bot.Bot, track *spoty.CurrentPlaying) {
	bio := s.fetchArtistInfoWithCache(ctx, track.Artist)
	msg := s.buildMessage(track, bio)
	s.sendToBot(ctx, b, track, msg)
}

func (s *spotifyPlayerHookImpl) OnOldTrackStillPlaying(ctx context.Context, b *bot.Bot, track *spoty.CurrentPlaying) {
	cached, _ := s.getCachedBio(track.Artist)
	var bio *lastfm.ArtistInfo
	if cached != nil {
		bio = cached.info
	}
	msg := s.buildMessage(track, bio)
	s.sendToBot(ctx, b, track, msg)
}

// ---------------------- Caching ----------------------

func (s *spotifyPlayerHookImpl) fetchArtistInfoWithCache(ctx context.Context, artist string) *lastfm.ArtistInfo {
	if cached, found := s.getCachedBio(artist); found {
		return cached.info
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, artistInfoFetchTimeout)
	defer cancel()

	langs := []string{"en", "ru"}
	for _, lang := range langs {
		info, err := s.lastFmClient.ArtistGetInfo(ctxTimeout, artist, lang)
		if err != nil {
			if s.onError != nil {
				s.onError(fmt.Errorf("fetch artist info %s: %w", artist, err))
			}
			continue
		}
		if info != nil && info.BioSummaryWithoutLinks() != "" {
			s.cacheBio(artist, info)
			return info
		}
	}

	s.cacheBio(artist, nil)
	return nil
}

func (s *spotifyPlayerHookImpl) getCachedBio(artist string) (*cachedBio, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cached, exists := s.bioCache[artist]
	if !exists || time.Now().After(cached.expiresAt) {
		return nil, false
	}
	return cached, true
}

func (s *spotifyPlayerHookImpl) cacheBio(artist string, info *lastfm.ArtistInfo) {
	message := s.formatCachedMessage(artist, info)

	s.mu.Lock()
	s.bioCache[artist] = &cachedBio{
		info:          info,
		expiresAt:     time.Now().Add(bioCacheTTL),
		cachedMessage: message,
	}
	s.mu.Unlock()
}

func (s *spotifyPlayerHookImpl) formatCachedMessage(artist string, info *lastfm.ArtistInfo) string {
	var message string
	if info != nil {
		message = s.formatBioAndLinks(info, artist)
	} else {
		message = s.formatDefaultLinks(artist)
	}
	message += "\n\n" + shared.TgText(shared.TotalRandomEmoji())
	return message
}

func (s *spotifyPlayerHookImpl) backgroundCacheCleaner() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			s.mu.Lock()
			for k, v := range s.bioCache {
				if now.After(v.expiresAt) {
					delete(s.bioCache, k)
				}
			}
			s.mu.Unlock()
		case <-s.shutdown:
			return
		}
	}
}

// ---------------------- Messaging ----------------------

func (s *spotifyPlayerHookImpl) buildIdleMessage() string {
	currentTime := shared.TimeToRuWithSeconds(time.Now())
	links := []string{
		shared.TgText("✉️ @dvdqr"),
		shared.TgText("✉️ oklocate@gmail.com"),
		"",
		shared.TgLink("💰 Донат (DA)", "https://donationalerts.com/r/oklookat"),
		shared.TgLink("💰 Донат (Boosty)", "https://boosty.to/oklookat/donate"),
		"",
		shared.TgLink("💻 GitHub", "https://github.com/oklookat"),
		"",
		shared.TgLink("🎧 Spotify", "https://open.spotify.com/user/60c4lc5cwaesypcv9mvzb1klf"),
		shared.TgLink("🎧 Last.fm", "https://last.fm/user/ndskmusic"),
		"",
		shared.TgLink("🍿 Кинопоиск", "https://kinopoisk.ru/user/166758523"),
	}
	return shared.TgText(currentTime) + "\n\n" + strings.Join(links, "\n")
}

func (s *spotifyPlayerHookImpl) buildMessage(track *spoty.CurrentPlaying, bio *lastfm.ArtistInfo) string {
	var sb strings.Builder
	sb.WriteString(shared.TgText(shared.TimeToRuWithSeconds(time.Now())))
	sb.WriteString("\n\n")
	sb.WriteString(s.formatTrackInfo(track))
	sb.WriteString("\n\n")
	sb.WriteString(s.formatBioSection(track, bio))
	sb.WriteString("\n")
	sb.WriteString(shared.TgLink("powered by oklookat/teletrack", "https://github.com/oklookat/teletrack"))
	return sb.String()
}

func (s *spotifyPlayerHookImpl) formatTrackInfo(track *spoty.CurrentPlaying) string {
	status := "▶️"
	if !track.Playing {
		status = "⏸️"
	}
	trackName := fmt.Sprintf("`%s`", shared.TgText(shared.EscapeMarkdownV2(track.Artist+" - "+track.Name)))
	progress := fmt.Sprintf("%s %s %s",
		s.formatTime(track.ProgressMs),
		s.formatProgressBar(track.ProgressMs, track.DurationMs),
		s.formatTime(track.DurationMs))

	return fmt.Sprintf("%s %s\n\n%s\n\n🔥 %d / 100",
		status, trackName, shared.TgText(progress), track.FullTrack.Popularity)
}

func (s *spotifyPlayerHookImpl) formatBioSection(track *spoty.CurrentPlaying, bio *lastfm.ArtistInfo) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if cached, ok := s.bioCache[track.Artist]; ok && cached != nil {
		return cached.cachedMessage
	} else if bio != nil {
		return s.formatBioAndLinks(bio, track.Artist)
	}
	return s.formatDefaultLinks(track.Artist)
}

// ---------------------- Utilities ----------------------

func (s *spotifyPlayerHookImpl) sendToBot(ctx context.Context, b *bot.Bot, track *spoty.CurrentPlaying, msg string) {
	if b == nil {
		return
	}
	opts := &models.LinkPreviewOptions{IsDisabled: bot.True()}
	if track != nil && track.CoverURL != nil && *track.CoverURL != "" {
		opts.IsDisabled = bot.False()
		opts.PreferLargeMedia = bot.True()
		opts.URL = track.CoverURL
	}

	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:             config.C.Telegram.ChatID,
		MessageID:          config.C.Telegram.MessageID,
		ParseMode:          models.ParseModeMarkdown,
		Text:               msg,
		LinkPreviewOptions: opts,
	})
	if err != nil && s.onError != nil {
		id := ""
		if track != nil {
			id = track.ID
		}
		s.onError(fmt.Errorf("sendToBot track %s: %w", id, err))
	}
}

func (s *spotifyPlayerHookImpl) formatTime(ms int) string {
	totalSec := ms / 1000
	return fmt.Sprintf("%02d:%02d", totalSec/60, totalSec%60)
}

func (s *spotifyPlayerHookImpl) formatProgressBar(progressMs, durationMs int) string {
	const blocks = 12
	if durationMs <= 0 {
		return strings.Repeat("░", blocks)
	}
	progressBlocks := int(float64(progressMs) / float64(durationMs) * blocks)
	if progressBlocks > blocks {
		progressBlocks = blocks
	}
	return fmt.Sprintf("[%s%s]", strings.Repeat("█", progressBlocks), strings.Repeat("░", blocks-progressBlocks))
}

func (s *spotifyPlayerHookImpl) formatBioAndLinks(info *lastfm.ArtistInfo, artist string) string {
	if info == nil {
		return s.formatDefaultLinks(artist)
	}
	var sb strings.Builder
	bioText := s.formatArtistBio(info)
	if bioText != "" {
		sb.WriteString(bioText)
		sb.WriteString("\n\n")
	}
	sb.WriteString(fmt.Sprintf("🔗 %s", shared.TgLink("Spotify", "https://open.spotify.com/artist/"+artist)))
	if info.Artist.URL != "" {
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("🔗 %s", shared.TgLink("Last.fm", info.Artist.URL)))
	}
	return strings.TrimSpace(sb.String())
}

func (s *spotifyPlayerHookImpl) formatDefaultLinks(artist string) string {
	return fmt.Sprintf("🔗 %s", shared.TgLink("Spotify", "https://open.spotify.com/artist/"+artist))
}

func (s *spotifyPlayerHookImpl) formatArtistBio(info *lastfm.ArtistInfo) string {
	if info == nil {
		return ""
	}
	bio := strings.Join(strings.Fields(info.BioSummaryWithoutLinks()), " ")
	if len(bio) < 20 {
		return ""
	}
	bio = smartTruncateSentences(bio, 300)
	return shared.TgText(shared.EscapeMarkdownV2(bio))
}

// ---------------------- Text Helpers ----------------------

var sentenceEnd = regexp.MustCompile(`(?m)(.*?[.!?])\s*`)

func smartTruncateSentences(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	sentences := sentenceEnd.FindAllString(text, -1)
	var sb strings.Builder
	for _, s := range sentences {
		if sb.Len()+len(s) > maxLen {
			break
		}
		sb.WriteString(s)
	}
	final := strings.TrimSpace(sb.String())
	if final == "" {
		return safeTruncate(text, maxLen)
	}
	last := final[len(final)-1]
	if last != '.' && last != '!' && last != '?' {
		final += "..."
	}
	return final
}

func safeTruncate(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	trimmed := text[:maxLen]
	if idx := strings.LastIndex(trimmed, " "); idx > 0 {
		trimmed = trimmed[:idx]
	}
	return strings.TrimSpace(trimmed) + "..."
}
