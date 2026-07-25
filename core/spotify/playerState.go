package spotify

// type PlayingMessage struct {
// 	*ArtistInfo
// 	*TrackInfo

// 	// 11:26:35 24.03.2026 (MSK)
// 	Time time.Time
// 	// Vibe emoji.
// 	Emoji string
// }

// type ArtistInfo struct {
// 	// Blood Orange
// 	Artist string
// 	// Artist bio, can be empty.
// 	Bio string
// 	// Example: link to last.fm. Can be empty.
// 	BioLink string
// }

// func newPlayerState(hooks PlayerStateHooks) *playerState {
// 	if hooks == nil {
// 		return nil
// 	}
// 	return &playerState{
// 		hooks:         hooks,
// 		cachedArtists: &expirable.LRU[string, *ArtistInfo]{},
// 	}
// }

// type playerState struct {
// 	prevMessage *PlayingMessage
// 	hooks       PlayerStateHooks

// 	cachedArtists *expirable.LRU[string, *ArtistInfo]
// }

// func (s *playerState) OnNothingPlaying(ctx context.Context) error {
// 	return s.hooks.OnNothingPlaying(ctx)
// }

// func (s *playerState) OnNewTrackPlayed(ctx context.Context, track *currentPlaying) error {
// 	if track == nil {
// 		if s.prevMessage == nil {
// 			return nil
// 		}
// 		return s.hooks.OnOldTrackStillPlaying(ctx, s.prevMessage)
// 	}

// 	msg, err := s.getPlayingMessage(ctx, track)
// 	if err != nil {
// 		if err := s.hooks.OnError(err); err != nil {
// 			return err
// 		}
// 	}

// 	s.prevMessage = msg
// 	s.hooks.OnNewTrackPlayed(ctx, s.prevMessage)
// 	return err

// }

// func (s *playerState) OnOldTrackStillPlaying(ctx context.Context, track *currentPlaying) error {
// 	if track == nil {
// 		if s.prevMessage == nil {
// 			return nil
// 		}
// 		return s.hooks.OnOldTrackStillPlaying(ctx, s.prevMessage)
// 	}

// 	msg, err := s.getPlayingMessage(ctx, track)
// 	if err != nil {
// 		if err := s.hooks.OnError(err); err != nil {
// 			return err
// 		}
// 	}

// 	s.prevMessage = msg
// 	s.hooks.OnOldTrackStillPlaying(ctx, s.prevMessage)
// 	return err
// }

// func (s *playerState) getPlayingMessage(ctx context.Context, track *currentPlaying) (*PlayingMessage, error) {
// 	artistInfo, err := s.getArtistInfo(ctx, track)
// 	if err != nil {
// 		if err := s.hooks.OnError(err); err != nil {
// 			return nil, err
// 		}
// 	}

// 	return &PlayingMessage{
// 		ArtistInfo: artistInfo,
// 		TrackInfo:  s.getTrackInfo(track),
// 		Time:       time.Now(),
// 		Emoji:      shared.TotalRandomEmoji(),
// 	}, err
// }

// func (s *playerState) getArtistInfo(ctx context.Context, track *currentPlaying) (*ArtistInfo, error) {
// 	if cached, ok := s.cachedArtists.Get(track.ArtistID); ok {
// 		return cached, nil
// 	}

// 	const (
// 		fetchTimeout = 5 * time.Second
// 	)
// 	ctxTimeout, cancel := context.WithTimeout(ctx, fetchTimeout)
// 	defer cancel()

// 	info, err := s.hooks.FetchArtistInfo(ctxTimeout, track.Artist)
// 	if err != nil {
// 		return nil, err
// 	}

// 	cached := &ArtistInfo{
// 		Artist:  track.Artist,
// 		Bio:     info.Bio,
// 		BioLink: info.BioLink,
// 	}

// 	s.cachedArtists.Add(track.ArtistID, cached)

// 	return cached, err
// }

// func (s *playerState) getTrackInfo(track *currentPlaying) *TrackInfo {
// 	if track == nil {
// 		return nil
// 	}
// 	info := &TrackInfo{
// 		ID:         track.ID,
// 		Track:      track.Name,
// 		TrackLink:  "https://open.spotify.com/track/" + track.ID,
// 		ProgressMs: track.ProgressMs,
// 		DurationMs: track.DurationMs,
// 		Playing:    track.Playing,
// 	}
// 	if track.CoverURL != nil && len(*track.CoverURL) > 0 {
// 		info.CoverURL = *track.CoverURL
// 	}

// 	return info
// }
