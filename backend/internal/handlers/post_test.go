package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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

// TestGetFeedSuccess tests successfully retrieving a feed
func TestGetFeedSuccess(t *testing.T) {
	db, mock, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("failed to setup mock db: %v", err)
	}
	defer db.Close()

	handler := NewPostHandler(db)
	sectionID := uuid.New()
	post1ID := uuid.New()
	post2ID := uuid.New()
	userID := uuid.New()
	now := time.Now()
	earlier := now.Add(-time.Hour)

	// Mock the posts query (returns 2 posts + 1 extra to determine hasMore)
	rows := mock.NewRows([]string{
		"id", "user_id", "section_id", "content",
		"created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at",
		"comment_count",
	}).AddRow(
		post1ID, userID, sectionID, "First post",
		now, nil, nil, nil,
		userID, "testuser", "test@example.com", nil, nil, false, now,
		2,
	).AddRow(
		post2ID, userID, sectionID, "Second post",
		earlier, nil, nil, nil,
		userID, "testuser", "test@example.com", nil, nil, false, earlier,
		0,
	)

	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	// Mock links queries
	linksRows := mock.NewRows([]string{"id", "url", "metadata", "created_at"})
	mock.ExpectQuery("SELECT id, url, metadata, created_at").WillReturnRows(linksRows)
	mock.ExpectQuery("SELECT id, url, metadata, created_at").WillReturnRows(linksRows)

	req, err := http.NewRequest("GET", "/api/v1/sections/"+sectionID.String()+"/feed", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler.GetFeed(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response models.FeedResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if len(response.Posts) != 2 {
		t.Errorf("expected 2 posts, got %d", len(response.Posts))
	}

	if response.HasMore {
		t.Errorf("expected hasMore to be false, got true")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestGetFeedWithCursor tests feed retrieval with cursor pagination
func TestGetFeedWithCursor(t *testing.T) {
	db, mock, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("failed to setup mock db: %v", err)
	}
	defer db.Close()

	handler := NewPostHandler(db)
	sectionID := uuid.New()
	postID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	// Mock the posts query
	rows := mock.NewRows([]string{
		"id", "user_id", "section_id", "content",
		"created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at",
		"comment_count",
	}).AddRow(
		postID, userID, sectionID, "Post after cursor",
		now, nil, nil, nil,
		userID, "testuser", "test@example.com", nil, nil, false, now,
		1,
	)

	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	// Mock links query
	linksRows := mock.NewRows([]string{"id", "url", "metadata", "created_at"})
	mock.ExpectQuery("SELECT id, url, metadata, created_at").WillReturnRows(linksRows)

	cursor := now.Add(-2 * time.Hour).Format("2006-01-02T15:04:05.000Z07:00")
	req, err := http.NewRequest("GET", "/api/v1/sections/"+sectionID.String()+"/feed?cursor="+cursor, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler.GetFeed(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response models.FeedResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if len(response.Posts) != 1 {
		t.Errorf("expected 1 post, got %d", len(response.Posts))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestGetFeedInvalidSectionID tests with invalid section ID format
func TestGetFeedInvalidSectionID(t *testing.T) {
	db, _, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("failed to setup mock db: %v", err)
	}
	defer db.Close()

	handler := NewPostHandler(db)

	req, err := http.NewRequest("GET", "/api/v1/sections/not-a-uuid/feed", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler.GetFeed(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "INVALID_SECTION_ID" {
		t.Errorf("expected code INVALID_SECTION_ID, got %s", response.Code)
	}
}

// TestRestorePostSuccess tests successfully restoring a deleted post by owner
func TestRestorePostSuccess(t *testing.T) {
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
	deletedAt := now.Add(-24 * time.Hour)

	// Mock the fetch deleted post query
	rows := mock.NewRows([]string{
		"id", "user_id", "section_id", "content",
		"created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at",
		"comment_count",
	}).AddRow(
		postID, userID, sectionID, "Test post content",
		now, nil, &deletedAt, nil,
		userID, "testuser", "test@example.com", nil, nil, false, now,
		0,
	)

	mock.ExpectQuery("SELECT").WithArgs(postID).WillReturnRows(rows)

	// Mock the restore update query
	updateRows := mock.NewRows([]string{
		"id", "user_id", "section_id", "content",
		"created_at", "updated_at", "deleted_at", "deleted_by_user_id",
	}).AddRow(
		postID, userID, sectionID, "Test post content",
		now, nil, nil, nil,
	)

	mock.ExpectQuery("UPDATE posts").WithArgs(postID).WillReturnRows(updateRows)

	// Mock the links query
	linksRows := mock.NewRows([]string{"id", "url", "metadata", "created_at"})
	mock.ExpectQuery("SELECT id, url, metadata, created_at").WithArgs(postID).WillReturnRows(linksRows)

	req, err := http.NewRequest("POST", "/api/v1/posts/"+postID.String()+"/restore", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Add user context
	req = req.WithContext(createTestUserContext(req.Context(), userID, "testuser", false))

	rr := httptest.NewRecorder()
	handler.RestorePost(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response models.RestorePostResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Post.ID != postID {
		t.Errorf("expected post id %s, got %s", postID, response.Post.ID)
	}

	if response.Post.DeletedAt != nil {
		t.Error("expected deleted_at to be nil after restore")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRestorePostByAdmin tests admin restoring a deleted post
func TestRestorePostByAdmin(t *testing.T) {
	db, mock, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("failed to setup mock db: %v", err)
	}
	defer db.Close()

	handler := NewPostHandler(db)
	postID := uuid.New()
	ownerID := uuid.New()
	adminID := uuid.New()
	sectionID := uuid.New()
	now := time.Now()
	deletedAt := now.Add(-8 * 24 * time.Hour) // 8 days ago (beyond 7-day window)

	// Mock the fetch deleted post query
	rows := mock.NewRows([]string{
		"id", "user_id", "section_id", "content",
		"created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at",
		"comment_count",
	}).AddRow(
		postID, ownerID, sectionID, "Test post content",
		now, nil, &deletedAt, nil,
		ownerID, "testuser", "test@example.com", nil, nil, false, now,
		0,
	)

	mock.ExpectQuery("SELECT").WithArgs(postID).WillReturnRows(rows)

	// Mock the restore update query
	updateRows := mock.NewRows([]string{
		"id", "user_id", "section_id", "content",
		"created_at", "updated_at", "deleted_at", "deleted_by_user_id",
	}).AddRow(
		postID, ownerID, sectionID, "Test post content",
		now, nil, nil, nil,
	)

	mock.ExpectQuery("UPDATE posts").WithArgs(postID).WillReturnRows(updateRows)

	// Mock the links query
	linksRows := mock.NewRows([]string{"id", "url", "metadata", "created_at"})
	mock.ExpectQuery("SELECT id, url, metadata, created_at").WithArgs(postID).WillReturnRows(linksRows)

	req, err := http.NewRequest("POST", "/api/v1/posts/"+postID.String()+"/restore", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Add admin user context
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "admin", true))

	rr := httptest.NewRecorder()
	handler.RestorePost(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRestorePostUnauthorized tests non-owner cannot restore
func TestRestorePostUnauthorized(t *testing.T) {
	db, mock, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("failed to setup mock db: %v", err)
	}
	defer db.Close()

	handler := NewPostHandler(db)
	postID := uuid.New()
	ownerID := uuid.New()
	otherUserID := uuid.New()
	sectionID := uuid.New()
	now := time.Now()
	deletedAt := now.Add(-24 * time.Hour)

	// Mock the fetch deleted post query
	rows := mock.NewRows([]string{
		"id", "user_id", "section_id", "content",
		"created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at",
		"comment_count",
	}).AddRow(
		postID, ownerID, sectionID, "Test post content",
		now, nil, &deletedAt, nil,
		ownerID, "testuser", "test@example.com", nil, nil, false, now,
		0,
	)

	mock.ExpectQuery("SELECT").WithArgs(postID).WillReturnRows(rows)

	req, err := http.NewRequest("POST", "/api/v1/posts/"+postID.String()+"/restore", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Add different user context
	req = req.WithContext(createTestUserContext(req.Context(), otherUserID, "otheruser", false))

	rr := httptest.NewRecorder()
	handler.RestorePost(rr, req)

	if status := rr.Code; status != http.StatusForbidden {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusForbidden)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "FORBIDDEN" {
		t.Errorf("expected code FORBIDDEN, got %s", response.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRestorePostPermanentlyDeleted tests cannot restore post older than 7 days
func TestRestorePostPermanentlyDeleted(t *testing.T) {
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
	deletedAt := now.Add(-8 * 24 * time.Hour) // 8 days ago

	// Mock the fetch deleted post query
	rows := mock.NewRows([]string{
		"id", "user_id", "section_id", "content",
		"created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at",
		"comment_count",
	}).AddRow(
		postID, userID, sectionID, "Test post content",
		now, nil, &deletedAt, nil,
		userID, "testuser", "test@example.com", nil, nil, false, now,
		0,
	)

	mock.ExpectQuery("SELECT").WithArgs(postID).WillReturnRows(rows)

	req, err := http.NewRequest("POST", "/api/v1/posts/"+postID.String()+"/restore", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Add user context
	req = req.WithContext(createTestUserContext(req.Context(), userID, "testuser", false))

	rr := httptest.NewRecorder()
	handler.RestorePost(rr, req)

	if status := rr.Code; status != http.StatusGone {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusGone)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Code != "POST_PERMANENTLY_DELETED" {
		t.Errorf("expected code POST_PERMANENTLY_DELETED, got %s", response.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// setupMockDB creates a mock database connection for testing
func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, error) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		return nil, nil, err
	}
	return db, mock, nil
}
