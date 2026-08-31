package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Server wraps a Renderer with an HTTP(S) listener.
//
// Prefer New + Register when mounting on an existing mux (for example the
// HTML status page). Prefer Start when the API should own a listen address.
//
// Start accepts an optional existing Renderer so HTML and API can share one
// state machine. When existing is nil, Start creates and owns the Renderer
// (and closes it on shutdown). When existing is non-nil, Start reuses it and
// does not close it.
type Server struct {
	*Renderer

	cfg          Config
	logger       *slog.Logger
	useTLS       bool
	ownsRenderer bool

	httpServer *http.Server
	ln         net.Listener
}

// Start listens on cfg.Addr, mounts the API under cfg.PathPrefix, and
// returns a Server that implements core.Renderer via the embedded Renderer.
//
// existing may be nil. See Server docs for ownership rules.
func Start(ctx context.Context, cfg Config, logger *slog.Logger, existing *Renderer) (*Server, error) {
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With(slog.String("component", "render_api"))

	addr := cfg.EffectiveAddr()
	prefix := cfg.EffectivePathPrefix()

	useTLS := cfg.TLSCertFile != "" || cfg.TLSKeyFile != ""
	if useTLS {
		if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" {
			return nil, fmt.Errorf("render_api: both tlsCertFile and tlsKeyFile are required for TLS")
		}
	}

	owns := existing == nil
	r := existing
	if r == nil {
		r = New(WithCORS(cfg.CORSConfig()))
	}

	mux := http.NewServeMux()
	r.Register(mux, prefix)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("render_api listen %s: %w", addr, err)
	}

	s := &Server{
		Renderer:     r,
		cfg:          cfg,
		logger:       logger,
		useTLS:       useTLS,
		ownsRenderer: owns,
		ln:           ln,
		httpServer: &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      0, // SSE streams stay open
			IdleTimeout:       60 * time.Second,
		},
	}

	go s.serve()
	go s.shutdownOnDone(ctx)

	scheme := "http"
	if useTLS {
		scheme = "https"
	}
	logger.Info("api listening",
		slog.String("addr", ln.Addr().String()),
		slog.String("prefix", prefix),
		slog.String("scheme", scheme),
		slog.Bool("tls", useTLS),
		slog.Bool("shared_renderer", !owns),
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
		s.logger.Error("api server stopped", slog.Any("error", err))
	}
}

func (s *Server) shutdownOnDone(ctx context.Context) {
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.logger.Warn("api shutdown", slog.Any("error", err))
	}
	if s.ownsRenderer {
		_ = s.Renderer.Close()
	}
}

// Addr returns the actual listen address (useful when Addr was ":0").
func (s *Server) Addr() string {
	if s.ln == nil {
		return s.cfg.EffectiveAddr()
	}
	return s.ln.Addr().String()
}
