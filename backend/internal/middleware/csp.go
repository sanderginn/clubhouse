package middleware

import (
	"net/http"
	"strings"
)

// CSPMiddleware adds a Content-Security-Policy header to reduce iframe injection risks.
// This policy is aligned with the embed domain whitelist in the links service.
func CSPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		csp := []string{
			"default-src 'self'",
			"script-src 'self' 'unsafe-inline' 'unsafe-eval' https://w.soundcloud.com",
			"style-src 'self' 'unsafe-inline'",
			"img-src 'self' data: https:",
			"frame-src 'self' https://www.youtube-nocookie.com https://open.spotify.com https://w.soundcloud.com https://bandcamp.com",
			"connect-src 'self' https://soundcloud.com https://api-widget.soundcloud.com",
			"font-src 'self'",
			"object-src 'none'",
			"base-uri 'self'",
			"form-action 'self'",
			"frame-ancestors 'none'",
		}

		w.Header().Set("Content-Security-Policy", strings.Join(csp, "; "))
		next.ServeHTTP(w, r)
	})
}
