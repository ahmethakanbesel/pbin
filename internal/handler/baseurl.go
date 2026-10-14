package handler

import (
	"context"
	"net/http"
	"strings"
)

type baseURLKey struct{}

// WithBaseURL makes publicURL (e.g. "https://pbin.example.com") the base for all
// absolute URLs built by handlers. An empty publicURL derives the base from each request.
func WithBaseURL(publicURL string, next http.Handler) http.Handler {
	publicURL = strings.TrimRight(publicURL, "/")
	if publicURL == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), baseURLKey{}, publicURL)))
	})
}

// baseURL returns the configured public URL, or scheme://host derived from the request.
// X-Forwarded-Proto is honoured so TLS-terminating reverse proxies produce https links.
func baseURL(r *http.Request) string {
	if u, ok := r.Context().Value(baseURLKey{}).(string); ok && u != "" {
		return u
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		proto = strings.ToLower(strings.TrimSpace(strings.Split(proto, ",")[0]))
		if proto == "http" || proto == "https" {
			scheme = proto
		}
	}
	return scheme + "://" + r.Host
}

// absURL turns a root-relative path into an absolute URL for this request.
// Already-absolute URLs are returned unchanged.
func absURL(r *http.Request, path string) string {
	if !strings.HasPrefix(path, "/") {
		return path
	}
	return baseURL(r) + path
}
