// Package html serves a public HTTP(S) status page for Teletrack.
//
// The page is a thin UI over the Teletrack API package
// (GET {prefix}/playing and {prefix}/events). State is owned by an
// *api.Renderer supplied by the caller so the HTML page and the standalone
// API renderer can share one state machine.
//
// Production example (config.json):
//
//	"renderers": ["telegram", "html", "api"],
//	"html": {
//	  "addr": ":443",
//	  "tlsCertFile": "/etc/teletrack/fullchain.pem",
//	  "tlsKeyFile":  "/etc/teletrack/privkey.pem"
//	}
//
// Bind addr to 0.0.0.0 (or omit the host) so remote clients can connect.
// Prefer terminating TLS at a reverse proxy if you already have one; the
// built-in TLS options are for simple single-binary deployments.
package html

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/oklookat/teletrack/core"
	"github.com/oklookat/teletrack/renderer/api"
)

// Config controls the public status HTTP(S) server.
type Config struct {
	// Addr is the listen address. Examples:
	//   "127.0.0.1:8787" — local only
	//   ":8787"          — all interfaces, HTTP
	//   ":443"           — all interfaces, typically with TLS
	// Default: "127.0.0.1:8787".
	Addr string `json:"addr"`

	// TLSCertFile and TLSKeyFile enable HTTPS when both are non-empty.
	// Paths are read at Start time (standard PEM certificate + private key).
	TLSCertFile string `json:"tlsCertFile,omitempty"`
	TLSKeyFile  string `json:"tlsKeyFile,omitempty"`

	// APIPathPrefix is where the shared Teletrack API is mounted on this server.
	// Default: "/api/v1/teletrack".
	APIPathPrefix string `json:"apiPathPrefix,omitempty"`

	// Logger is runtime-only (not loaded from JSON).
	Logger *slog.Logger `json:"-"`
}

// Server is an HTTP(S) UI over a shared *api.Renderer.
//
// When the HTML renderer is the only consumer of that Renderer, Server
// implements core.Renderer by delegating to it. When the standalone API
// renderer also uses the same instance, the loader registers the shared
// Renderer once with core; Server is then only an HTTP front-end.
type Server struct {
	api    *api.Renderer
	cfg    Config
	logger *slog.Logger
	useTLS bool

	// ownsAPI is true when this Server created the Renderer and must Close it.
	ownsAPI bool

	httpServer *http.Server
	ln         net.Listener
}

// Start listens on cfg.Addr and serves the status page.
//
// apiRenderer is the shared Teletrack API state. If nil, Start creates a
// private Renderer (HTML-only deployments) and owns its lifecycle.
//
// The page always talks to the API mounted on this same server under
// cfg.APIPathPrefix so same-origin fetch/SSE work without CORS.
func Start(ctx context.Context, cfg Config, apiRenderer *api.Renderer) (*Server, error) {
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:8787"
	}
	if cfg.APIPathPrefix == "" {
		cfg.APIPathPrefix = api.DefaultPathPrefix
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With(slog.String("component", "render_html"))

	useTLS := cfg.TLSCertFile != "" || cfg.TLSKeyFile != ""
	if useTLS {
		if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" {
			return nil, fmt.Errorf("render_html: both tlsCertFile and tlsKeyFile are required for TLS")
		}
	}

	ownsAPI := apiRenderer == nil
	if apiRenderer == nil {
		apiRenderer = api.New(api.WithCORS(api.DefaultCORSConfig()))
	}

	s := &Server{
		api:     apiRenderer,
		cfg:     cfg,
		logger:  logger,
		useTLS:  useTLS,
		ownsAPI: ownsAPI,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	apiRenderer.Register(mux, cfg.APIPathPrefix)

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("render_html listen %s: %w", cfg.Addr, err)
	}
	s.ln = ln

	handler := securityHeaders(mux)

	s.httpServer = &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      0, // SSE streams stay open
		IdleTimeout:       60 * time.Second,
	}

	go s.serve()
	go s.shutdownOnDone(ctx)

	scheme := "http"
	if useTLS {
		scheme = "https"
	}
	logger.Info("html status UI listening",
		slog.String("addr", ln.Addr().String()),
		slog.String("scheme", scheme),
		slog.String("api_prefix", cfg.APIPathPrefix),
		slog.Bool("tls", useTLS),
		slog.Bool("shared_api", !ownsAPI),
	)

	return s, nil
}

func (s *Server) serve() {
	var err error
	if s.useTLS {
		err = s.httpServer.ServeTLS(s.ln, s.cfg.TLSCertFile, s.cfg.TLSKeyFile)
	} else {
		err = s.httpServer.Serve(s.ln)
	}
	if err != nil && err != http.ErrServerClosed {
		s.logger.Error("render_html server stopped", slog.Any("error", err))
	}
}

func (s *Server) shutdownOnDone(ctx context.Context) {
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.logger.Warn("render_html shutdown", slog.Any("error", err))
	}
	if s.ownsAPI {
		_ = s.api.Close()
	}
}

// Addr returns the actual listen address (useful when Addr was ":0").
func (s *Server) Addr() string {
	if s.ln == nil {
		return s.cfg.Addr
	}
	return s.ln.Addr().String()
}

// API returns the Teletrack API renderer used by this page.
func (s *Server) API() *api.Renderer {
	return s.api
}

// UpdatePlaying implements core.Renderer by delegating to the shared API.
func (s *Server) UpdatePlaying(ctx context.Context, msg *core.PlayingMessage) error {
	return s.api.UpdatePlaying(ctx, msg)
}

// UpdateIdle implements core.Renderer by delegating to the shared API.
func (s *Server) UpdateIdle(ctx context.Context, msg *core.PlayingMessage) error {
	return s.api.UpdateIdle(ctx, msg)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(indexHTML(s.cfg.APIPathPrefix)))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", strings.Join([]string{
			"default-src 'self'",
			"img-src 'self' https: data:",
			"style-src 'self' 'unsafe-inline'",
			"script-src 'self' 'unsafe-inline'",
			"connect-src 'self'",
			"base-uri 'none'",
			"frame-ancestors 'none'",
		}, "; "))
		next.ServeHTTP(w, r)
	})
}
