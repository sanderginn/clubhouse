package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestListSectionsSuccess(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

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
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

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
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create a test section
	sectionID := testutil.CreateTestSection(t, db, "Test Section", "general")

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
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

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
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

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

func TestGetSectionLinksSuccess(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "sectionlinks", "sectionlinks@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Links Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Post with links")

	now := time.Now().UTC()
	older := now.Add(-2 * time.Hour)
	newer := now.Add(-1 * time.Hour)

	insertTestSectionLink(t, db, postID, "https://example.com/older", nil, older)
	insertTestSectionLink(t, db, postID, "https://example.com/newer", nil, newer)

	handler := NewSectionHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/sections/"+sectionID+"/links?limit=1", nil)
	w := httptest.NewRecorder()

	handler.GetSectionLinks(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.SectionLinksResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(response.Links))
	}

	if response.Links[0].URL != "https://example.com/newer" {
		t.Errorf("expected newest link first, got %s", response.Links[0].URL)
	}

	if response.NextCursor == nil || !response.HasMore {
		t.Fatalf("expected next cursor and hasMore true")
	}
}

func TestGetSectionLinksInvalidCursor(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	sectionID := testutil.CreateTestSection(t, db, "Links Section", "general")
	handler := NewSectionHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/sections/"+sectionID+"/links?cursor=not-a-time", nil)
	w := httptest.NewRecorder()

	handler.GetSectionLinks(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Code != "INVALID_CURSOR" {
		t.Errorf("expected error code INVALID_CURSOR, got %s", response.Code)
	}
}

func TestGetSectionLinksNotFound(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewSectionHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/sections/00000000-0000-0000-0000-000000000000/links", nil)
	w := httptest.NewRecorder()

	handler.GetSectionLinks(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Code != "SECTION_NOT_FOUND" {
		t.Errorf("expected error code SECTION_NOT_FOUND, got %s", response.Code)
	}
}

func TestGetSectionLinksInvalidID(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewSectionHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/sections/not-a-uuid/links", nil)
	w := httptest.NewRecorder()

	handler.GetSectionLinks(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Code != "INVALID_SECTION_ID" {
		t.Errorf("expected error code INVALID_SECTION_ID, got %s", response.Code)
	}
}

func insertTestSectionLink(t *testing.T, db *sql.DB, postID, url string, metadata map[string]interface{}, createdAt time.Time) {
	t.Helper()

	var metadataValue interface{}
	if metadata != nil {
		bytes, err := json.Marshal(metadata)
		if err != nil {
			t.Fatalf("failed to marshal metadata: %v", err)
		}
		metadataValue = string(bytes)
	}

	_, err := db.Exec(
		`INSERT INTO links (id, post_id, url, metadata, created_at) VALUES (gen_random_uuid(), $1, $2, $3, $4)`,
		postID, url, metadataValue, createdAt,
	)
	if err != nil {
		t.Fatalf("failed to insert link: %v", err)
	}
}
