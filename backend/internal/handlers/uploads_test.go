package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
)

func TestUploadImageSuccess(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("CLUBHOUSE_UPLOAD_DIR", tempDir)

	handler := NewUploadHandler()
	userID := uuid.New()

	payload := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	req := newMultipartRequest(t, "file", "image.png", "image/png", payload)
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, &services.Session{UserID: userID})
	req = req.WithContext(ctx)

	recorder := httptest.NewRecorder()
	handler.UploadImage(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response models.ImageUploadResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !strings.HasPrefix(response.URL, "/api/v1/uploads/") {
		t.Fatalf("expected upload URL, got %q", response.URL)
	}

	relativePath := strings.TrimPrefix(response.URL, "/api/v1/uploads/")
	if relativePath == response.URL {
		t.Fatalf("expected upload URL to include prefix, got %q", response.URL)
	}

	filePath := filepath.Join(tempDir, filepath.FromSlash(relativePath))
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("expected uploaded file to exist: %v", err)
	}
}

func TestUploadImageRejectsInvalidType(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("CLUBHOUSE_UPLOAD_DIR", tempDir)

	handler := NewUploadHandler()
	userID := uuid.New()

	req := newMultipartRequest(t, "file", "notes.txt", "text/plain", []byte("hello"))
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, &services.Session{UserID: userID})
	req = req.WithContext(ctx)

	recorder := httptest.NewRecorder()
	handler.UploadImage(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if response.Code != "INVALID_FILE_TYPE" {
		t.Fatalf("expected INVALID_FILE_TYPE, got %q", response.Code)
	}
}

func TestUploadImageRejectsLargeFile(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("CLUBHOUSE_UPLOAD_DIR", tempDir)
	t.Setenv("CLUBHOUSE_UPLOAD_MAX_BYTES", "5")

	handler := NewUploadHandler()
	userID := uuid.New()

	payload := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	req := newMultipartRequest(t, "file", "image.png", "image/png", payload)
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, &services.Session{UserID: userID})
	req = req.WithContext(ctx)

	recorder := httptest.NewRecorder()
	handler.UploadImage(recorder, req)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, recorder.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if response.Code != "FILE_TOO_LARGE" {
		t.Fatalf("expected FILE_TOO_LARGE, got %q", response.Code)
	}
}

func newMultipartRequest(t *testing.T, fieldName, filename, contentType string, payload []byte) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", "form-data; name=\""+fieldName+"\"; filename=\""+filename+"\"")
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("failed to create multipart part: %v", err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatalf("failed to write multipart payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}
