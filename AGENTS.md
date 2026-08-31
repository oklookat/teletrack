# AGENTS.md

Guide for automated coding agents working on this repository.

## Project

**teletrack** is a Go daemon that polls music sources and pushes “now playing” status to one or more outputs (renderers).

- Module: `github.com/oklookat/teletrack`
- Language: Go 1.25+
- Style: [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- Docs and comments: English

## Commands

```bash
go test ./...
go build -o teletrack .
go build -ldflags "-X github.com/oklookat/teletrack/shared.Version=$(git describe --tags --always)" .
```

Config schema: `config.schema.json`. Keep it in sync when changing `config.Config` or renderer configs.

## High-level flow

```
main
  config.Boot
  loader.Load            # players + artist getters
  loader.LoadRenderers   # telegram / html / api
  cache.NewSQLiteCache
  core.Teletrack.Start   # poll loop → Renderer.UpdatePlaying / UpdateIdle
```

Core never depends on concrete Spotify/Telegram/HTTP types. It only uses interfaces in `core`.

## Packages

| Package | Responsibility |
|---------|----------------|
| `main` | Flags, signals, wiring |
| `config` | Load/save JSON + env overrides (`TELETRACK_*`) |
| `loader` | Build players, bios, renderers from config |
| `core` | Playback state machine, bio fetch, fan-out to renderers |
| `cache` | SQLite KV with TTL + LRU |
| `spotify`, `lastfm`, `listenbrainz` | Players and/or bio sources |
| `renderer/telegram` | Telegram bot + Markdown status message |
| `renderer/api` | Shared JSON/SSE state + optional standalone HTTP server |
| `renderer/html` | Status page UI over the API |
| `shared`, `ago` | Small helpers |

## Core interfaces

Defined in `core/abstract.go`:

- `Player` — `GetPlaying(ctx) (Track, error)`
- `ArtistGetter` — bios
- `Renderer` — `UpdatePlaying` / `UpdateIdle` (must be safe for concurrent use)
- `Track`, `ArtistInfo` — data interfaces

`PlayingMessage` (`core/message.go`) is the internal payload. Public HTTP JSON is `api.State` (`renderer/api/types.go`). Do not leak `core` types into the public API contract.

## Renderers

Config field `renderers` is a list: `telegram`, `html`, `api`. Any combination is valid.

### Shared API state

`html` and `api` share **one** `*api.Renderer` when both are enabled (`loader/renderers.go`):

1. Create `sharedAPI := api.New(...)`
2. `api.Start(..., sharedAPI)` if `api` is enabled (standalone listen address)
3. `html.Start(..., sharedAPI)` if `html` is enabled (page + same handlers on HTML addr)
4. Register `sharedAPI` **once** with core

Do not give HTML its own independent state when the API renderer is also active.

Ownership:

- Creator of `*api.Renderer` closes it on shutdown
- `api.Start` / `html.Start` take an optional existing renderer; if non-nil they do not close it

### API endpoints

Default prefix: `/api/v1/teletrack`

| Method | Path | Body |
|--------|------|------|
| GET | `{prefix}/playing` | `api.State` JSON |
| GET | `{prefix}/events` | SSE, `event: state`, data = `api.State` |

Idle responses still include the last track, cover, and bio when known (same idea as Telegram idle).

TLS: set both `tlsCertFile` and `tlsKeyFile` on `html` or `api` config. CORS only on standalone `api` (for cross-origin frontends).

### HTML page

- Static HTML/CSS/JS in `renderer/html/page.go`
- Consumes the API mounted on the **same** HTML server (same origin)
- Responsive layout; idle shows last track + cover + bio

### Telegram

Separate from API/HTML. Implements `core.Renderer` via the messenger adapter. Keep Markdown/escaping helpers in `renderer/telegram`.

## Config

- File + env; env wins
- Service names: `spotify`, `lastFm`, `listenBrainz`, `telegram`, `html`, `api`
- Adding a renderer or config field: update `config.Config`, defaults in `config.C`, `config.schema.json`, and README

## Conventions

- Prefer small packages with clear boundaries
- Wrap errors with `%w` and context
- Use `log/slog` with a component field where useful
- No global mutable state beyond `config.C` (prefer the `*Config` returned from `Boot`)
- Renderers must not block the core loop on slow clients (API SSE uses non-blocking client buffers)
- Tests: table-driven where practical; HTTP tests bind `127.0.0.1:0`

## What not to do

- Do not duplicate playback state between HTML and API
- Do not expose `core.PlayingMessage` as the public HTTP schema
- Do not add heavy frameworks for HTTP; stdlib `net/http` is intentional
- Do not break the JSON field names of `api.State` without a versioned path change

## Useful entry points

| Task | Start here |
|------|------------|
| Poll / idle logic | `core/main.go` |
| Wire new player | `loader/main.go` + new package implementing `core.Player` |
| Wire new renderer | `loader/renderers.go` + `config.Service*` |
| Public JSON/SSE | `renderer/api/` |
| Status page UI | `renderer/html/page.go` |
| Telegram text | `renderer/telegram/teletrackRenderer.go` |
