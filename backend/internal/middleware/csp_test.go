package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCSPMiddlewareSetsHeader(t *testing.T) {
	handler := CSPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	header := rec.Header().Get("Content-Security-Policy")
	if header == "" {
		t.Fatal("expected Content-Security-Policy header to be set")
	}
	if !strings.Contains(header, "frame-src") {
		t.Fatalf("expected frame-src directive, got %q", header)
	}
	if !strings.Contains(header, "https://www.youtube-nocookie.com") {
		t.Fatalf("expected youtube-nocookie in CSP, got %q", header)
	}
}
