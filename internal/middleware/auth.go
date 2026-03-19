// Package middleware provides HTTP middleware for the pbin service.
package middleware

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/ahmethakanbesel/pbin/internal/config"
)

// BasicAuth returns a middleware that enforces HTTP Basic Auth when cfg.Enabled is true.
// When disabled, the handler is called directly with no overhead.
// Credential comparison uses crypto/subtle.ConstantTimeCompare to prevent timing attacks.
func BasicAuth(cfg config.AuthConfig, next http.Handler) http.Handler {
	if !cfg.Enabled {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")

		username, password, ok := r.BasicAuth()
		if !ok {
			writeUnauthorized(w)
			return
		}

		usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(cfg.Username))
		passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(cfg.Password))
		if usernameMatch != 1 || passwordMatch != 1 {
			writeUnauthorized(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// writeUnauthorized sends a 401 JSON response with a WWW-Authenticate header.
func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="pbin"`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": "authentication required"})
}
