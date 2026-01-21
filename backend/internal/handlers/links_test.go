package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
)

func TestPreviewLinkSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!doctype html><html><head>
      <meta property="og:title" content="Test Title" />
      <meta property="og:description" content="Test Description" />
      <meta property="og:image" content="/image.png" />
      </head><body></body></html>`))
	}))
	defer server.Close()

	handler := NewLinkHandler()
	body, _ := json.Marshal(models.LinkPreviewRequest{URL: server.URL})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links/preview", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	session := &services.Session{
		UserID:   uuid.New(),
		Username: "tester",
		IsAdmin:  false,
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, session)
	req = req.WithContext(ctx)

	recorder := httptest.NewRecorder()
	handler.PreviewLink(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var response models.LinkPreviewResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Metadata["title"] != "Test Title" {
		t.Fatalf("expected title metadata, got %v", response.Metadata["title"])
	}
	if response.Metadata["url"] != server.URL {
		t.Fatalf("expected url metadata, got %v", response.Metadata["url"])
	}
}

func TestPreviewLinkDisabled(t *testing.T) {
	configService := services.GetConfigService()
	disabled := false
	configService.UpdateConfig(&disabled)
	defer func() {
		enabled := true
		configService.UpdateConfig(&enabled)
	}()

	handler := NewLinkHandler()
	body, _ := json.Marshal(models.LinkPreviewRequest{URL: "https://example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links/preview", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	session := &services.Session{
		UserID:   uuid.New(),
		Username: "tester",
		IsAdmin:  false,
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, session)
	req = req.WithContext(ctx)

	recorder := httptest.NewRecorder()
	handler.PreviewLink(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", recorder.Code)
	}
}

func TestPreviewLinkInvalidBody(t *testing.T) {
	handler := NewLinkHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links/preview", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")

	session := &services.Session{
		UserID:   uuid.New(),
		Username: "tester",
		IsAdmin:  false,
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, session)
	req = req.WithContext(ctx)

	recorder := httptest.NewRecorder()
	handler.PreviewLink(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestPreviewLinkMethodNotAllowed(t *testing.T) {
	handler := NewLinkHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/links/preview", nil)
	recorder := httptest.NewRecorder()

	handler.PreviewLink(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", recorder.Code)
	}
}
