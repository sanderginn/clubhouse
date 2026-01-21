package middleware

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Middleware func(http.Handler) http.Handler

// ContextKey is a custom type for context keys
type ContextKey string

const (
	// UserContextKey is the key for storing user info in context
	UserContextKey ContextKey = "user"
	// SessionIDContextKey is the key for storing session ID in context
	SessionIDContextKey ContextKey = "session_id"
	// SectionIDContextKey is the key for storing the current section ID in context
	SectionIDContextKey ContextKey = "section_id"
)

var uuidPattern = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

func normalizeRoute(path string) string {
	return uuidPattern.ReplaceAllString(path, "{id}")
}

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

// Observability middleware for tracing and metrics.
func Observability(next http.Handler) http.Handler {
	handler := otelhttp.NewHandler(
		next,
		"http.server",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		}),
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		handler.ServeHTTP(recorder, r)
		route := normalizeRoute(r.URL.Path)
		observability.RecordHTTPRequest(r.Context(), r.Method, route, recorder.statusCode, time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := r.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

// RequireAuth middleware validates session cookie and injects user context
func RequireAuth(redis *redis.Client) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get session cookie
			cookie, err := r.Cookie("session_id")
			if err != nil {
				writeAuthError(r.Context(), w, http.StatusUnauthorized, "NO_SESSION", "Authentication required")
				return
			}

			sessionID := cookie.Value
			sessionService := services.NewSessionService(redis)

			// Validate session
			session, err := sessionService.GetSession(r.Context(), sessionID)
			if err != nil {
				writeAuthError(r.Context(), w, http.StatusUnauthorized, "INVALID_SESSION", "Session not found or expired")
				return
			}

			// Inject session and user into context
			ctx := context.WithValue(r.Context(), SessionIDContextKey, sessionID)
			ctx = context.WithValue(ctx, UserContextKey, session)
			if sectionID := strings.TrimSpace(r.Header.Get("X-Section-ID")); sectionID != "" {
				if parsedID, err := uuid.Parse(sectionID); err == nil {
					ctx = context.WithValue(ctx, SectionIDContextKey, parsedID)
				}
			}

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
				writeAuthError(r.Context(), w, http.StatusUnauthorized, "NO_SESSION", "Authentication required")
				return
			}

			sessionID := cookie.Value
			sessionService := services.NewSessionService(redis)

			// Validate session
			session, err := sessionService.GetSession(r.Context(), sessionID)
			if err != nil {
				writeAuthError(r.Context(), w, http.StatusUnauthorized, "INVALID_SESSION", "Session not found or expired")
				return
			}

			// Check if user is admin
			if !session.IsAdmin {
				writeAuthError(r.Context(), w, http.StatusForbidden, "ADMIN_REQUIRED", "Admin access required")
				return
			}

			// Inject session and user into context
			ctx := context.WithValue(r.Context(), SessionIDContextKey, sessionID)
			ctx = context.WithValue(ctx, UserContextKey, session)
			if sectionID := strings.TrimSpace(r.Header.Get("X-Section-ID")); sectionID != "" {
				if parsedID, err := uuid.Parse(sectionID); err == nil {
					ctx = context.WithValue(ctx, SectionIDContextKey, parsedID)
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// writeAuthError is a helper to write authentication error responses
func writeAuthError(ctx context.Context, w http.ResponseWriter, statusCode int, code string, message string) {
	userID := ""
	if id, err := GetUserIDFromContext(ctx); err == nil {
		userID = id.String()
	}
	observability.LogError(ctx, observability.ErrorLog{
		Message:    message,
		Code:       code,
		StatusCode: statusCode,
		UserID:     userID,
	})

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
