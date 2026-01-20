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

func TestGetSectionSuccess(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

	var sectionID string
	err = db.QueryRow(`SELECT id FROM sections LIMIT 1`).Scan(&sectionID)
	if err != nil {
		t.Skip("no sections in database")
	}

	handler := NewSectionHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/sections/"+sectionID, nil)
	w := httptest.NewRecorder()

	handler.GetSection(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.GetSectionResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}
}

func TestGetSectionNotFound(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

	handler := NewSectionHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/sections/00000000-0000-0000-0000-000000000000", nil)
	w := httptest.NewRecorder()

	handler.GetSection(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "SECTION_NOT_FOUND" {
		t.Errorf("expected error code SECTION_NOT_FOUND, got %s", response.Code)
	}
}

func TestGetSectionInvalidID(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

	handler := NewSectionHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/sections/invalid-id", nil)
	w := httptest.NewRecorder()

	handler.GetSection(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "INVALID_SECTION_ID" {
		t.Errorf("expected error code INVALID_SECTION_ID, got %s", response.Code)
	}
}
