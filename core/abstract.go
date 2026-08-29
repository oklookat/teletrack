package core

import (
	"context"
	"time"
)

type Player interface {
	// Get current playing track.
	GetPlaying(ctx context.Context) (TrackInfoer, error)
}

type Messenger interface {
	UpdatePlaying(context.Context, *PlayingMessage) error
	UpdateIdle(context.Context, *PlayingMessage) error
}

type ArtistGetter interface {
	// First lang is preferred. Other langs for fallback if first lang doesnt have bio.

	// Langs format:

	// ISO639-2 code (see https://www.loc.gov/standards/iso639-2/php/code_list.php
	GetArtistInfo(ctx context.Context, artist string, langs []string) (ArtistInfoer, error)
}

type ArtistInfoer interface {
	// Link to artist or artist bio on BioService.
	Link() string
	// Artist bio. Must be short, cleaned.
	Bio() string
	// Example: Last.fm
	BioService() string
}

type TrackInfoer interface {
	// md5 of "ArtistName:TrackName". Generates automatically by SetID
	ID() string

	// Playing now?
	Playing() bool

	// Artist name.
	Artist() string

	// Track name.
	Track() string

	// Track link if available. Example: link to Spotify.
	TrackLink() string

	// Service name where track link placed. Example: "Spotify".
	TrackLinkService() string

	// Track cover URL. Can be emprty.
	CoverURL() string

	// Track current progress in ms. Nil if unsupported.
	ProgressMs() *int

	// Track total duration in ms. Nil if unsupported.
	DurationMs() *int

	// Current time (last track). Nil if unsupported.
	Time() *time.Time
}

type dummyArtistInfo struct {
}

func (d dummyArtistInfo) Link() string {
	return ""
}

func (d dummyArtistInfo) Bio() string {
	return ""
}

func (d dummyArtistInfo) BioService() string {
	return ""
}
