// Package api exposes Teletrack playback state over HTTP.
//
// It implements core.Renderer and can run in two modes:
//
//   - Embedded: New() + Handler()/Register() on an existing mux
//     (used by the HTML status page).
//   - Standalone: Start() binds its own listen address when "api" is
//     listed in config.renderers.
//
// Public endpoints (under PathPrefix, default /api/v1/teletrack):
//
//	GET /playing  — JSON snapshot of the current state
//	GET /events   — Server-Sent Events stream (event: state)
//
// The JSON contract is stable for third-party frontends. When idle, the
// last track (and bio/cover) remain present so clients can mirror the
// Telegram idle message.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/oklookat/teletrack/core"
)

const (
	DefaultPathPrefix = "/api/v1/teletrack"

	defaultClientBuffer = 16
	defaultWriteTimeout = 10 * time.Second

	eventState = "state"
)

// Renderer implements core.Renderer and exposes Teletrack state over HTTP.
//
// It provides:
//
//	GET /playing - current state snapshot
//	GET /events  - Server-Sent Events stream
//
// Renderer is safe for concurrent use.
type Renderer struct {
	mu sync.RWMutex

	state State

	clients map[*client]struct{}

	clientBuffer int
	writeTimeout time.Duration

	closed bool

	cors CORSConfig
}

// Option configures Renderer.
type Option func(*Renderer)

// WithClientBuffer configures the per-client event buffer.
//
// If a client cannot consume events quickly enough to fit into this buffer,
// that client is disconnected. The Teletrack core loop is never blocked by a
// slow HTTP client.
func WithClientBuffer(size int) Option {
	return func(r *Renderer) {
		if size > 0 {
			r.clientBuffer = size
		}
	}
}

// WithWriteTimeout configures the maximum time allowed for an SSE write.
//
// Note that the normal net/http ResponseWriter does not provide a portable
// write deadline, so this option is primarily useful to implementations that
// provide a deadline-aware ResponseWriter.
func WithWriteTimeout(timeout time.Duration) Option {
	return func(r *Renderer) {
		if timeout > 0 {
			r.writeTimeout = timeout
		}
	}
}

// WithCORS configures CORS.
func WithCORS(config CORSConfig) Option {
	return func(r *Renderer) {
		r.cors = config
	}
}

// New creates a new API Renderer.
func New(options ...Option) *Renderer {
	now := time.Now().UTC()
	r := &Renderer{
		state: State{
			Playing:   false,
			Idle:      true,
			Time:      now,
			UpdatedAt: now,
		},
		clients:      make(map[*client]struct{}),
		clientBuffer: defaultClientBuffer,
		writeTimeout: defaultWriteTimeout,
		cors:         DefaultCORSConfig(),
	}

	for _, option := range options {
		if option != nil {
			option(r)
		}
	}

	return r
}

// UpdatePlaying implements core.Renderer.
func (r *Renderer) UpdatePlaying(
	ctx context.Context,
	msg *core.PlayingMessage,
) error {
	if msg == nil {
		return errors.New("nil playing message")
	}

	state := stateFromMessage(msg, true)

	return r.update(state)
}

// UpdateIdle implements core.Renderer.
func (r *Renderer) UpdateIdle(
	ctx context.Context,
	msg *core.PlayingMessage,
) error {
	if msg == nil {
		return errors.New("nil playing message")
	}

	state := stateFromMessage(msg, false)

	return r.update(state)
}

func (r *Renderer) update(state State) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}

	event := encodeSSEEvent(eventState, payload)

	r.mu.Lock()

	if r.closed {
		r.mu.Unlock()
		return errors.New("api renderer is closed")
	}

	r.state = state

	clients := make([]*client, 0, len(r.clients))

	for c := range r.clients {
		clients = append(clients, c)
	}

	r.mu.Unlock()

	// Never hold r.mu while talking to clients.
	//
	// A slow/disconnected browser must not block Teletrack.
	for _, c := range clients {
		if !c.enqueue(event) {
			r.removeClient(c)
		}
	}

	return nil
}

// State returns the latest known state.
func (r *Renderer) State() State {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.state
}

// Close closes all active SSE connections and prevents new connections.
//
// It is safe to call multiple times.
func (r *Renderer) Close() error {
	r.mu.Lock()

	if r.closed {
		r.mu.Unlock()
		return nil
	}

	r.closed = true

	clients := make([]*client, 0, len(r.clients))

	for c := range r.clients {
		clients = append(clients, c)
	}

	r.clients = make(map[*client]struct{})

	r.mu.Unlock()

	for _, c := range clients {
		c.close()
	}

	return nil
}

// Handler returns the HTTP handler for the API.
//
// Endpoints:
//
//	GET /playing
//	GET /events
func (r *Renderer) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/playing", r.handlePlaying)
	mux.HandleFunc("/events", r.handleEvents)

	return r.withMiddleware(mux)
}

// Register mounts the API on an existing ServeMux.
//
// Example:
//
//	apiRenderer.Register(mux, "/api/v1/teletrack")
func (r *Renderer) Register(
	mux *http.ServeMux,
	prefix string,
) {
	if mux == nil {
		return
	}

	prefix = normalizePrefix(prefix)

	mux.Handle(
		prefix+"/playing",
		r.withMiddleware(http.HandlerFunc(r.handlePlaying)),
	)

	mux.Handle(
		prefix+"/events",
		r.withMiddleware(http.HandlerFunc(r.handleEvents)),
	)
}

func (r *Renderer) handlePlaying(
	w http.ResponseWriter,
	req *http.Request,
) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	writeJSON(w, http.StatusOK, r.State())
}

func (r *Renderer) handleEvents(
	w http.ResponseWriter,
	req *http.Request,
) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	if r.isClosed() {
		writeJSON(
			w,
			http.StatusServiceUnavailable,
			ErrorResponse{
				Error: "api renderer is shutting down",
			},
		)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(
			w,
			http.StatusInternalServerError,
			ErrorResponse{
				Error: "HTTP streaming is not supported",
			},
		)
		return
	}

	w.Header().Set(
		"Content-Type",
		"text/event-stream; charset=utf-8",
	)
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	c := newClient(r.clientBuffer)

	// Register client and take a snapshot atomically.
	r.mu.Lock()

	if r.closed {
		r.mu.Unlock()

		writeJSON(
			w,
			http.StatusServiceUnavailable,
			ErrorResponse{
				Error: "api renderer is shutting down",
			},
		)
		return
	}

	r.clients[c] = struct{}{}

	state := r.state

	r.mu.Unlock()

	defer r.removeClient(c)

	// Send the current state immediately.
	//
	// This means a newly connected frontend does not have to wait for the
	// next Teletrack tick.
	payload, err := json.Marshal(state)
	if err != nil {
		return
	}

	if !writeSSE(
		w,
		flusher,
		encodeSSEEvent(eventState, payload),
	) {
		return
	}

	// Heartbeat prevents idle SSE connections from being closed by some
	// proxies/load balancers.
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-req.Context().Done():
			return

		case <-c.done:
			return

		case event, ok := <-c.events:
			if !ok {
				return
			}

			if !writeSSE(w, flusher, event) {
				return
			}

		case <-heartbeat.C:
			if !writeSSE(
				w,
				flusher,
				[]byte(": heartbeat\n\n"),
			) {
				return
			}
		}
	}
}

func (r *Renderer) removeClient(c *client) {
	if c == nil {
		return
	}

	r.mu.Lock()

	delete(r.clients, c)

	r.mu.Unlock()

	c.close()
}

func (r *Renderer) isClosed() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.closed
}

func (r *Renderer) withMiddleware(
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		req *http.Request,
	) {
		r.cors.apply(w, req)

		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, req)
	})
}

func normalizePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)

	if prefix == "" {
		return ""
	}

	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}

	return strings.TrimRight(prefix, "/")
}
