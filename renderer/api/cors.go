package api

import (
	"net/http"
	"strings"
)

// CORSConfig controls cross-origin access to the API.
type CORSConfig struct {
	AllowedOrigins []string

	AllowedMethods []string

	AllowedHeaders []string

	AllowCredentials bool
}

// DefaultCORSConfig disables cross-origin requests unless an origin is
// explicitly configured.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{},

		AllowedMethods: []string{
			http.MethodGet,
			http.MethodOptions,
		},

		AllowedHeaders: []string{
			"Accept",
			"Content-Type",
			"Last-Event-ID",
		},

		AllowCredentials: false,
	}
}

func (c CORSConfig) apply(
	w http.ResponseWriter,
	req *http.Request,
) {
	origin := req.Header.Get("Origin")

	if origin == "" {
		return
	}

	if !c.isAllowedOrigin(origin) {
		return
	}

	w.Header().Set(
		"Access-Control-Allow-Origin",
		origin,
	)

	w.Header().Add("Vary", "Origin")

	if len(c.AllowedMethods) > 0 {
		w.Header().Set(
			"Access-Control-Allow-Methods",
			strings.Join(c.AllowedMethods, ", "),
		)
	}

	if len(c.AllowedHeaders) > 0 {
		w.Header().Set(
			"Access-Control-Allow-Headers",
			strings.Join(c.AllowedHeaders, ", "),
		)
	}

	if c.AllowCredentials {
		w.Header().Set(
			"Access-Control-Allow-Credentials",
			"true",
		)
	}
}

func (c CORSConfig) isAllowedOrigin(
	origin string,
) bool {
	for _, allowed := range c.AllowedOrigins {
		if allowed == "*" {
			return true
		}

		if allowed == origin {
			return true
		}
	}

	return false
}
