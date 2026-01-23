package middleware

import (
	"net/http"

	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/services"
)

// RequireCSRF middleware validates CSRF tokens on state-changing requests
func RequireCSRF(redis *redis.Client) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only validate CSRF for state-changing methods
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// Get CSRF token from header
			csrfToken := r.Header.Get("X-CSRF-Token")
			if csrfToken == "" {
				writeAuthError(r.Context(), w, http.StatusForbidden, "CSRF_TOKEN_REQUIRED", "CSRF token is required for this request")
				return
			}

			// Get session from context (injected by RequireAuth middleware)
			session, err := GetUserFromContext(r.Context())
			if err != nil {
				writeAuthError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
				return
			}

			sessionID, err := GetSessionIDFromContext(r.Context())
			if err != nil {
				writeAuthError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Session ID not found")
				return
			}

			// Validate CSRF token
			csrfService := services.NewCSRFService(redis)
			if err := csrfService.ValidateToken(r.Context(), csrfToken, sessionID, session.UserID); err != nil {
				writeAuthError(r.Context(), w, http.StatusForbidden, "INVALID_CSRF_TOKEN", "Invalid or expired CSRF token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
