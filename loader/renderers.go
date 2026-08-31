package loader

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/oklookat/teletrack/config"
	"github.com/oklookat/teletrack/core"
	"github.com/oklookat/teletrack/renderer/api"
	"github.com/oklookat/teletrack/renderer/html"
	"github.com/oklookat/teletrack/renderer/telegram"
	"github.com/oklookat/teletrack/shared"
)

// RendererDeps holds objects that renderer factories cannot construct alone.
type RendererDeps struct {
	// TelegramBot is required when "telegram" is listed in cfg.Renderers.
	TelegramBot *telegram.TelegramBot
}

// LoadRenderers constructs status renderers from cfg.Renderers.
//
// Supported renderers:
//
//   - telegram — edits a status message in Telegram
//   - html     — public status page over the Teletrack API
//   - api      — standalone HTTP API (JSON + SSE)
//
// HTML and API share a single *api.Renderer when both are enabled so there is
// one state machine and one set of SSE clients. Core receives that shared
// renderer only once. Each HTTP server still binds its own address.
func LoadRenderers(ctx context.Context, cfg *config.Config, deps RendererDeps) ([]core.Renderer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	enabled := shared.Unique(cfg.Renderers)
	if len(enabled) == 0 {
		slog.Warn("check your config: renderers is empty; status will not be displayed anywhere")
		return nil, nil
	}

	wantTelegram := containsService(enabled, config.ServiceTelegram)
	wantHTML := containsService(enabled, config.ServiceHTML)
	wantAPI := containsService(enabled, config.ServiceAPI)

	out := make([]core.Renderer, 0, len(enabled))

	if wantTelegram {
		if deps.TelegramBot == nil {
			return nil, fmt.Errorf("renderer %q requires a telegram bot", config.ServiceTelegram)
		}
		r := telegram.NewMessenger(deps.TelegramBot)
		if r == nil {
			return nil, fmt.Errorf("renderer %q: failed to create telegram messenger", config.ServiceTelegram)
		}
		out = append(out, r)
		slog.Info("renderer loaded", "name", config.ServiceTelegram)
	}

	// One shared API state for html and/or api.
	var sharedAPI *api.Renderer
	if wantHTML || wantAPI {
		opts := []api.Option{}
		if wantAPI {
			apiCfg := cfg.API
			if apiCfg == nil {
				apiCfg = api.DefaultConfig()
			}
			opts = append(opts, api.WithCORS(apiCfg.CORSConfig()))
		} else {
			opts = append(opts, api.WithCORS(api.DefaultCORSConfig()))
		}
		sharedAPI = api.New(opts...)
	}

	if wantAPI {
		apiCfg := cfg.API
		if apiCfg == nil {
			apiCfg = api.DefaultConfig()
		}
		srv, err := api.Start(ctx, *apiCfg, slog.Default(), sharedAPI)
		if err != nil {
			return nil, fmt.Errorf("renderer %q: %w", config.ServiceAPI, err)
		}
		slog.Info("renderer loaded",
			"name", config.ServiceAPI,
			"addr", srv.Addr(),
			"prefix", apiCfg.EffectivePathPrefix(),
		)
	}

	if wantHTML {
		htmlCfg := cfg.HTML
		if htmlCfg == nil {
			htmlCfg = &html.Config{}
		}
		page, err := html.Start(ctx, html.Config{
			Addr:          htmlCfg.Addr,
			TLSCertFile:   htmlCfg.TLSCertFile,
			TLSKeyFile:    htmlCfg.TLSKeyFile,
			APIPathPrefix: htmlCfg.APIPathPrefix,
			Logger:        slog.Default(),
		}, sharedAPI)
		if err != nil {
			return nil, fmt.Errorf("renderer %q: %w", config.ServiceHTML, err)
		}
		slog.Info("renderer loaded", "name", config.ServiceHTML, "addr", page.Addr())
	}

	// Register shared API state once with core.
	// HTML-only: html.Server also implements core.Renderer and would duplicate
	// updates if both were appended; prefer the shared *api.Renderer when present.
	if sharedAPI != nil {
		out = append(out, sharedAPI)
	}

	for _, name := range enabled {
		switch name {
		case config.ServiceTelegram, config.ServiceHTML, config.ServiceAPI:
			// handled above
		default:
			slog.Warn("unknown renderer in config", "renderer", name)
		}
	}

	return out, nil
}

func containsService(list []config.Service, want config.Service) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
