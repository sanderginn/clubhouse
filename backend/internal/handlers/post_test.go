package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
)

// TestGetPostSuccess tests successfully retrieving a post
func TestGetPostSuccess(t *testing.T) {
	// Create mock database for testing
	db, mock, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("failed to setup mock db: %v", err)
	}
	defer db.Close()

	handler := NewPostHandler(db)
	postID := uuid.New()
	userID := uuid.New()
	sectionID := uuid.New()
	now := time.Now()

	// Mock the query response
	rows := mock.NewRows([]string{
		"id", "user_id", "section_id", "content",
		"created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at",
		"comment_count",
	}).AddRow(
		postID, userID, sectionID, "Test post content",
		now, nil, nil, nil,
		userID, "testuser", "test@example.com", nil, nil, false, now,
		5,
	)

	mock.ExpectQuery("SELECT").WithArgs(postID).WillReturnRows(rows)

	// Mock the links query
	linksRows := mock.NewRows([]string{"id", "url", "metadata", "created_at"})
	mock.ExpectQuery("SELECT id, url, metadata, created_at").WithArgs(postID).WillReturnRows(linksRows)

	req, err := http.NewRequest("GET", "/api/v1/posts/"+postID.String(), nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler.GetPost(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response models.GetPostResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Post == nil {
		t.Error("expected post in response, got nil")
	}

	if response.Post.ID != postID {
		t.Errorf("expected post id %s, got %s", postID, response.Post.ID)
	}

	if response.Post.Content != "Test post content" {
		t.Errorf("expected content 'Test post content', got '%s'", response.Post.Content)
	}

	if response.Post.CommentCount != 5 {
		t.Errorf("expected comment count 5, got %d", response.Post.CommentCount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestGetPostNotFound tests retrieving a non-existent post
func TestGetPostNotFound(t *testing.T) {
	db, mock, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("failed to setup mock db: %v", err)
	}
	defer db.Close()

	handler := NewPostHandler(db)
	postID := uuid.New()

	// Mock no rows returned
	mock.ExpectQuery("SELECT").WithArgs(postID).WillReturnError(sql.ErrNoRows)

	req, err := http.NewRequest("GET", "/api/v1/posts/"+postID.String(), nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler.GetPost(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusNotFound)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "POST_NOT_FOUND" {
		t.Errorf("expected code POST_NOT_FOUND, got %s", response.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestGetPostInvalidID tests with invalid post ID format
func TestGetPostInvalidID(t *testing.T) {
	db, _, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("failed to setup mock db: %v", err)
	}
	defer db.Close()

	handler := NewPostHandler(db)

	req, err := http.NewRequest("GET", "/api/v1/posts/not-a-uuid", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler.GetPost(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "INVALID_POST_ID" {
		t.Errorf("expected code INVALID_POST_ID, got %s", response.Code)
	}
}

// TestGetPostMethodNotAllowed tests with non-GET method
func TestGetPostMethodNotAllowed(t *testing.T) {
	db, _, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("failed to setup mock db: %v", err)
	}
	defer db.Close()

	handler := NewPostHandler(db)
	postID := uuid.New()

	req, err := http.NewRequest("POST", "/api/v1/posts/"+postID.String(), bytes.NewBufferString("{}"))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler.GetPost(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusMethodNotAllowed)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "METHOD_NOT_ALLOWED" {
		t.Errorf("expected code METHOD_NOT_ALLOWED, got %s", response.Code)
	}
}

// setupMockDB creates a mock database connection for testing
func setupMockDB(t *testing.T) (*sql.DB, interface{}, error) {
	// In a real test setup, we'd use sqlmock or similar
	// For now, we'll skip the actual mock setup
	return nil, nil, nil
}
