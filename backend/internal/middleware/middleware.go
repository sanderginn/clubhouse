package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/services"
)

type Middleware func(http.Handler) http.Handler

// ContextKey is a custom type for context keys
type ContextKey string

const (
	// UserContextKey is the key for storing user info in context
	UserContextKey ContextKey = "user"
	// SessionIDContextKey is the key for storing session ID in context
	SessionIDContextKey ContextKey = "session_id"
)

// ChainMiddleware applies middleware in order
func ChainMiddleware(handler http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// RequestID middleware adds a unique request ID to the context
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		ctx := context.WithValue(r.Context(), "request_id", requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Observability middleware for tracing and metrics (placeholder)
func Observability(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Add OpenTelemetry tracing
		next.ServeHTTP(w, r)
	})
}

// RequireAuth middleware validates session cookie and injects user context
func RequireAuth(redis *redis.Client) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get session cookie
			cookie, err := r.Cookie("session_id")
			if err != nil {
				writeAuthError(w, http.StatusUnauthorized, "NO_SESSION", "Authentication required")
				return
			}

			sessionID := cookie.Value
			sessionService := services.NewSessionService(redis)

			// Validate session
			session, err := sessionService.GetSession(r.Context(), sessionID)
			if err != nil {
				writeAuthError(w, http.StatusUnauthorized, "INVALID_SESSION", "Session not found or expired")
				return
			}

			// Inject session and user into context
			ctx := context.WithValue(r.Context(), SessionIDContextKey, sessionID)
			ctx = context.WithValue(ctx, UserContextKey, session)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin middleware validates that the authenticated user is an admin
func RequireAdmin(redis *redis.Client) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// First, validate authentication
			cookie, err := r.Cookie("session_id")
			if err != nil {
				writeAuthError(w, http.StatusUnauthorized, "NO_SESSION", "Authentication required")
				return
			}

			sessionID := cookie.Value
			sessionService := services.NewSessionService(redis)

			// Validate session
			session, err := sessionService.GetSession(r.Context(), sessionID)
			if err != nil {
				writeAuthError(w, http.StatusUnauthorized, "INVALID_SESSION", "Session not found or expired")
				return
			}

			// Check if user is admin
			if !session.IsAdmin {
				writeAuthError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Admin access required")
				return
			}

			// Inject session and user into context
			ctx := context.WithValue(r.Context(), SessionIDContextKey, sessionID)
			ctx = context.WithValue(ctx, UserContextKey, session)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// writeAuthError is a helper to write authentication error responses
func writeAuthError(w http.ResponseWriter, statusCode int, code string, message string) {
	type errorResponse struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(errorResponse{
		Error: message,
		Code:  code,
	})
}
