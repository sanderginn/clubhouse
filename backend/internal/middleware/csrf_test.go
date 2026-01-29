package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/services"
	"github.com/sanderginn/clubhouse/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedisForMiddleware(t *testing.T) *redis.Client {
	client := testutil.GetTestRedis(t)

	// Clean up test data after each test
	t.Cleanup(func() {
		testutil.CleanupRedis(t)
	})

	return client
}

func TestRequireCSRF_AllowGET(t *testing.T) {
	client := setupTestRedisForMiddleware(t)
	middleware := RequireCSRF(client)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "success", rec.Body.String())
}

func TestRequireCSRF_AllowHEAD(t *testing.T) {
	client := setupTestRedisForMiddleware(t)
	middleware := RequireCSRF(client)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodHead, "/test", nil)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireCSRF_AllowOPTIONS(t *testing.T) {
	client := setupTestRedisForMiddleware(t)
	middleware := RequireCSRF(client)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireCSRF_RejectPOSTWithoutToken(t *testing.T) {
	client := setupTestRedisForMiddleware(t)
	middleware := RequireCSRF(client)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var response map[string]string
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "CSRF_TOKEN_REQUIRED", response["code"])
	assert.Contains(t, response["error"], "CSRF token is required")
}

func TestRequireCSRF_RejectPOSTWithoutAuth(t *testing.T) {
	client := setupTestRedisForMiddleware(t)
	middleware := RequireCSRF(client)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-CSRF-Token", "some-token")
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var response map[string]string
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "UNAUTHORIZED", response["code"])
}

func TestRequireCSRF_AcceptValidToken(t *testing.T) {
	client := setupTestRedisForMiddleware(t)
	csrfMiddleware := RequireCSRF(client)
	csrfService := services.NewCSRFService(client)

	sessionID := uuid.New().String()
	userID := uuid.New()

	// Create a mock session in context
	session := &services.Session{
		ID:       sessionID,
		UserID:   userID,
		Username: "testuser",
		IsAdmin:  false,
	}

	// Generate CSRF token
	ctx := context.Background()
	token, err := csrfService.GenerateToken(ctx, sessionID, userID)
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-CSRF-Token", token)

	// Inject session into context
	ctx = context.WithValue(ctx, UserContextKey, session)
	ctx = context.WithValue(ctx, SessionIDContextKey, sessionID)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	csrfMiddleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "success", rec.Body.String())
}

func TestRequireCSRF_RejectInvalidToken(t *testing.T) {
	client := setupTestRedisForMiddleware(t)
	csrfMiddleware := RequireCSRF(client)

	sessionID := uuid.New().String()
	userID := uuid.New()

	session := &services.Session{
		ID:       sessionID,
		UserID:   userID,
		Username: "testuser",
		IsAdmin:  false,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-CSRF-Token", "invalid-token")

	ctx := context.WithValue(context.Background(), UserContextKey, session)
	ctx = context.WithValue(ctx, SessionIDContextKey, sessionID)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	csrfMiddleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var response map[string]string
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_CSRF_TOKEN", response["code"])
}

func TestRequireCSRF_RejectPUTWithoutToken(t *testing.T) {
	client := setupTestRedisForMiddleware(t)
	middleware := RequireCSRF(client)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPut, "/test", nil)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireCSRF_RejectPATCHWithoutToken(t *testing.T) {
	client := setupTestRedisForMiddleware(t)
	middleware := RequireCSRF(client)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPatch, "/test", nil)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireCSRF_RejectDELETEWithoutToken(t *testing.T) {
	client := setupTestRedisForMiddleware(t)
	middleware := RequireCSRF(client)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodDelete, "/test", nil)
	rec := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}
