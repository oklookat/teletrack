package api

// Config controls the standalone HTTP API server.
//
// When "api" is listed in config.renderers, teletrack starts an HTTP(S)
// server that exposes the current playback state for third-party sites
// and widgets.
//
// Example (config.json):
//
//	"renderers": ["api"],
//	"api": {
//	  "addr": "0.0.0.0:8790",
//	  "pathPrefix": "/api/v1/teletrack",
//	  "tlsCertFile": "/etc/teletrack/fullchain.pem",
//	  "tlsKeyFile": "/etc/teletrack/privkey.pem",
//	  "cors": {
//	    "allowedOrigins": ["https://example.com"]
//	  }
//	}
type Config struct {
	// Addr is the listen address for the standalone API server.
	// Examples: "127.0.0.1:8790", ":8790", "0.0.0.0:8790".
	// Default: "127.0.0.1:8790".
	Addr string `json:"addr"`

	// PathPrefix is the URL prefix for API routes.
	// Endpoints become {PathPrefix}/playing and {PathPrefix}/events.
	// Default: "/api/v1/teletrack".
	PathPrefix string `json:"pathPrefix,omitempty"`

	// TLSCertFile and TLSKeyFile enable HTTPS when both are non-empty.
	TLSCertFile string `json:"tlsCertFile,omitempty"`
	TLSKeyFile  string `json:"tlsKeyFile,omitempty"`

	// CORS controls cross-origin access. Empty allowedOrigins means
	// no cross-origin requests are allowed (same-origin only).
	CORS *CORSConfigJSON `json:"cors,omitempty"`
}

// CORSConfigJSON is the JSON-serializable form of CORSConfig.
type CORSConfigJSON struct {
	AllowedOrigins   []string `json:"allowedOrigins,omitempty"`
	AllowedMethods   []string `json:"allowedMethods,omitempty"`
	AllowedHeaders   []string `json:"allowedHeaders,omitempty"`
	AllowCredentials bool     `json:"allowCredentials,omitempty"`
}

// DefaultConfig returns sane defaults for the standalone API server.
func DefaultConfig() *Config {
	return &Config{
		Addr:       "127.0.0.1:8790",
		PathPrefix: DefaultPathPrefix,
	}
}

// CORSConfig builds the runtime CORS settings from JSON config.
func (c *Config) CORSConfig() CORSConfig {
	base := DefaultCORSConfig()
	if c == nil || c.CORS == nil {
		return base
	}
	j := c.CORS
	if len(j.AllowedOrigins) > 0 {
		base.AllowedOrigins = j.AllowedOrigins
	}
	if len(j.AllowedMethods) > 0 {
		base.AllowedMethods = j.AllowedMethods
	}
	if len(j.AllowedHeaders) > 0 {
		base.AllowedHeaders = j.AllowedHeaders
	}
	base.AllowCredentials = j.AllowCredentials
	return base
}

// EffectivePathPrefix returns PathPrefix or the default.
func (c *Config) EffectivePathPrefix() string {
	if c == nil || c.PathPrefix == "" {
		return DefaultPathPrefix
	}
	return normalizePrefix(c.PathPrefix)
}

// EffectiveAddr returns Addr or the default.
func (c *Config) EffectiveAddr() string {
	if c == nil || c.Addr == "" {
		return "127.0.0.1:8790"
	}
	return c.Addr
}
