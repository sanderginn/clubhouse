package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanderginn/clubhouse/internal/models"
)

func TestListSectionsSuccess(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

	handler := NewSectionHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/sections", nil)
	w := httptest.NewRecorder()

	handler.ListSections(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.ListSectionsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Sections == nil {
		t.Error("expected sections to not be nil")
	}
}

func TestListSectionsMethodNotAllowed(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

	handler := NewSectionHandler(db)

	req := httptest.NewRequest("POST", "/api/v1/sections", nil)
	w := httptest.NewRecorder()

	handler.ListSections(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "METHOD_NOT_ALLOWED" {
		t.Errorf("expected error code METHOD_NOT_ALLOWED, got %s", response.Code)
	}
}
