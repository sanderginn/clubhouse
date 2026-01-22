package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestPreviewLinkSuccess(t *testing.T) {
	htmlBody := `<!doctype html><html><head>
      <meta property="og:title" content="Test Title" />
      <meta property="og:description" content="Test Description" />
      <meta property="og:image" content="/image.png" />
      </head><body></body></html>`

	previousTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() != "93.184.216.34" {
			return nil, errors.New("unexpected host")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
			Body:       io.NopCloser(strings.NewReader(htmlBody)),
			Request:    r,
		}, nil
	})
	defer func() {
		http.DefaultTransport = previousTransport
	}()

	handler := NewLinkHandler()
	body, _ := json.Marshal(models.LinkPreviewRequest{URL: "http://93.184.216.34/test"})
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
	if response.Metadata["url"] != "http://93.184.216.34/test" {
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

func TestPreviewLinkURLTooLong(t *testing.T) {
	handler := NewLinkHandler()
	longURL := "https://example.com/" + strings.Repeat("a", 2030)
	body, _ := json.Marshal(models.LinkPreviewRequest{URL: longURL})
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

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}

	var errResp models.ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}

	if errResp.Code != "URL_TOO_LONG" {
		t.Fatalf("expected error code URL_TOO_LONG, got %s", errResp.Code)
	}
}
