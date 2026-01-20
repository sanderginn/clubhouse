package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
)

func TestSearchMethodNotAllowed(t *testing.T) {
	handler := &SearchHandler{}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", nil)
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, status)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Code != "METHOD_NOT_ALLOWED" {
		t.Fatalf("expected code METHOD_NOT_ALLOWED, got %s", response.Code)
	}
}

func TestSearchMissingQuery(t *testing.T) {
	handler := &SearchHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search", nil)
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, status)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Code != "QUERY_REQUIRED" {
		t.Fatalf("expected code QUERY_REQUIRED, got %s", response.Code)
	}
}

func TestSearchInvalidScope(t *testing.T) {
	handler := &SearchHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test&scope=invalid", nil)
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, status)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Code != "INVALID_SCOPE" {
		t.Fatalf("expected code INVALID_SCOPE, got %s", response.Code)
	}
}

func TestSearchSectionScopeMissingSectionID(t *testing.T) {
	handler := &SearchHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test&scope=section", nil)
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, status)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Code != "SECTION_ID_REQUIRED" {
		t.Fatalf("expected code SECTION_ID_REQUIRED, got %s", response.Code)
	}
}

func TestSearchSectionScopeUsesContextSectionID(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	handler := NewSearchHandler(db)

	query := "section search"
	limit := 2
	postID := uuid.New()
	commentID := uuid.New()
	userID := uuid.New()
	sectionID := uuid.New()
	postCreated := time.Now()
	commentCreated := time.Now()
	userCreated := time.Now()

	searchRows := sqlmock.NewRows([]string{"result_type", "id", "rank"}).
		AddRow("post", postID, 0.42).
		AddRow("comment", commentID, 0.31)

	mock.ExpectQuery(regexp.QuoteMeta("WITH q AS")).
		WithArgs(query, sectionID, limit).
		WillReturnRows(searchRows)

	postRows := sqlmock.NewRows([]string{
		"id", "user_id", "section_id", "content", "created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at", "comment_count",
	}).AddRow(
		postID, userID, sectionID, "post content", postCreated, nil, nil, nil,
		userID, "alice", "alice@example.com", nil, nil, false, userCreated, 0,
	)

	mock.ExpectQuery(regexp.QuoteMeta("FROM posts p")).
		WithArgs(postID).
		WillReturnRows(postRows)

	mock.ExpectQuery(regexp.QuoteMeta("FROM links")).
		WithArgs(postID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url", "metadata", "created_at"}))

	commentRows := sqlmock.NewRows([]string{
		"id", "user_id", "post_id", "parent_comment_id", "content", "created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at",
	}).AddRow(
		commentID, userID, postID, nil, "comment content", commentCreated, nil, nil, nil,
		userID, "alice", "alice@example.com", nil, nil, false, userCreated,
	)

	mock.ExpectQuery(regexp.QuoteMeta("FROM comments c")).
		WithArgs(commentID).
		WillReturnRows(commentRows)

	mock.ExpectQuery(regexp.QuoteMeta("FROM links")).
		WithArgs(commentID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url", "metadata", "created_at"}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=section%20search&scope=section&limit=2", nil)
	ctx := context.WithValue(req.Context(), middleware.SectionIDContextKey, sectionID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}

	var response models.SearchResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(response.Results))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}

func TestSearchInvalidLimit(t *testing.T) {
	handler := &SearchHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test&scope=global&limit=abc", nil)
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, status)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Code != "INVALID_LIMIT" {
		t.Fatalf("expected code INVALID_LIMIT, got %s", response.Code)
	}
}

func TestSearchSuccessGlobal(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	handler := NewSearchHandler(db)

	query := "hello world"
	limit := 2
	postID := uuid.New()
	commentID := uuid.New()
	userID := uuid.New()
	sectionID := uuid.New()
	postCreated := time.Now()
	commentCreated := time.Now()
	userCreated := time.Now()

	searchRows := sqlmock.NewRows([]string{"result_type", "id", "rank"}).
		AddRow("post", postID, 0.42).
		AddRow("comment", commentID, 0.31)

	mock.ExpectQuery(regexp.QuoteMeta("WITH q AS")).
		WithArgs(query, limit).
		WillReturnRows(searchRows)

	postRows := sqlmock.NewRows([]string{
		"id", "user_id", "section_id", "content", "created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at", "comment_count",
	}).AddRow(
		postID, userID, sectionID, "post content", postCreated, nil, nil, nil,
		userID, "alice", "alice@example.com", nil, nil, false, userCreated, 0,
	)

	mock.ExpectQuery(regexp.QuoteMeta("FROM posts p")).
		WithArgs(postID).
		WillReturnRows(postRows)

	mock.ExpectQuery(regexp.QuoteMeta("FROM links")).
		WithArgs(postID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url", "metadata", "created_at"}))

	commentRows := sqlmock.NewRows([]string{
		"id", "user_id", "post_id", "parent_comment_id", "content", "created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at",
	}).AddRow(
		commentID, userID, postID, nil, "comment content", commentCreated, nil, nil, nil,
		userID, "alice", "alice@example.com", nil, nil, false, userCreated,
	)

	mock.ExpectQuery(regexp.QuoteMeta("FROM comments c")).
		WithArgs(commentID).
		WillReturnRows(commentRows)

	mock.ExpectQuery(regexp.QuoteMeta("FROM links")).
		WithArgs(commentID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url", "metadata", "created_at"}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=hello%20world&scope=global&limit=2", nil)
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}

	var response models.SearchResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(response.Results))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}
