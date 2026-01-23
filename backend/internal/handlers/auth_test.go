package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sanderginn/clubhouse/internal/models"
)

type stubAuthRateLimiter struct {
	allowed bool
	err     error
	calls   int
	lastIP  string
	lastIDs []string
}

func (s *stubAuthRateLimiter) Allow(_ context.Context, ip string, identifiers []string) (bool, error) {
	s.calls++
	s.lastIP = ip
	s.lastIDs = identifiers
	return s.allowed, s.err
}

func TestLoginRateLimited(t *testing.T) {
	limiter := &stubAuthRateLimiter{allowed: false}
	handler := &AuthHandler{rateLimiter: limiter}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"TestUser","password":"Password123"}`))
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", w.Code)
	}

	var resp models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "RATE_LIMITED" {
		t.Fatalf("expected RATE_LIMITED code, got %s", resp.Code)
	}
}

func TestRegisterRateLimited(t *testing.T) {
	limiter := &stubAuthRateLimiter{allowed: false}
	handler := &AuthHandler{rateLimiter: limiter}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"username":"TestUser","email":"test@example.com","password":"Password123"}`))
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", w.Code)
	}

	var resp models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "RATE_LIMITED" {
		t.Fatalf("expected RATE_LIMITED code, got %s", resp.Code)
	}
}
