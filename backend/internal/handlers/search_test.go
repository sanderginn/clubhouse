package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
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

func TestSearchQueryTooLong(t *testing.T) {
	handler := &SearchHandler{}

	query := strings.Repeat("a", maxSearchQueryLength+1)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q="+query, nil)
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, status)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Code != "QUERY_TOO_LONG" {
		t.Fatalf("expected code QUERY_TOO_LONG, got %s", response.Code)
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

func TestSearchInvalidQuery(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	handler := NewSearchHandler(db)

	query := "the and or"
	mock.ExpectQuery(regexp.QuoteMeta("SELECT plainto_tsquery('english', $1)::text")).
		WithArgs(query).
		WillReturnRows(sqlmock.NewRows([]string{"plainto_tsquery"}).AddRow(""))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=the%20and%20or&scope=global", nil)
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, status)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Code != "QUERY_INVALID" {
		t.Fatalf("expected code QUERY_INVALID, got %s", response.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
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
	limit := 3
	postID := uuid.New()
	commentID := uuid.New()
	linkID := uuid.New()
	userID := uuid.New()
	sectionID := uuid.New()
	postCreated := time.Now()
	commentCreated := time.Now()
	userCreated := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT plainto_tsquery('english', $1)::text")).
		WithArgs(query).
		WillReturnRows(sqlmock.NewRows([]string{"plainto_tsquery"}).AddRow("search"))

	searchRows := sqlmock.NewRows([]string{"result_type", "id", "rank"}).
		AddRow("post", postID, 0.42).
		AddRow("comment", commentID, 0.36).
		AddRow("link_metadata", linkID, 0.31)

	mock.ExpectQuery(regexp.QuoteMeta("WITH q AS")).
		WithArgs(query, sectionID, limit).
		WillReturnRows(searchRows)

	postRows := sqlmock.NewRows([]string{
		"id", "user_id", "section_id", "content", "created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at", "comment_count", "type",
	}).AddRow(
		postID, userID, sectionID, "post content", postCreated, nil, nil, nil,
		userID, "alice", "alice@example.com", nil, nil, false, userCreated, 0, "general",
	)

	mock.ExpectQuery(regexp.QuoteMeta("FROM posts p")).
		WithArgs(postID).
		WillReturnRows(postRows)

	mock.ExpectQuery(regexp.QuoteMeta("FROM links")).
		WithArgs(postID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url", "metadata", "created_at"}))

	mock.ExpectQuery(regexp.QuoteMeta("FROM post_images")).
		WithArgs(postID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "image_url", "position", "caption", "alt_text", "created_at"}))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT emoji, COUNT")).
		WithArgs(postID).
		WillReturnRows(sqlmock.NewRows([]string{"emoji", "count"}))

	commentRows := sqlmock.NewRows([]string{
		"id", "user_id", "post_id", "section_id", "parent_comment_id", "image_id", "timestamp_seconds", "content", "contains_spoiler", "created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at",
	}).AddRow(
		commentID, userID, postID, sectionID, nil, nil, nil, "comment content", false, commentCreated, nil, nil, nil,
		userID, "alice", "alice@example.com", nil, nil, false, userCreated,
	)

	mock.ExpectQuery(regexp.QuoteMeta("FROM comments c")).
		WithArgs(commentID).
		WillReturnRows(commentRows)

	mock.ExpectQuery(regexp.QuoteMeta("FROM links")).
		WithArgs(commentID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url", "metadata", "created_at"}))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT emoji, COUNT")).
		WithArgs(commentID).
		WillReturnRows(sqlmock.NewRows([]string{"emoji", "count"}))

	linkRows := sqlmock.NewRows([]string{"id", "url", "metadata", "post_id", "comment_id"}).
		AddRow(linkID, "https://example.com", []byte(`{"title":"Example"}`), postID, nil)

	mock.ExpectQuery(`FROM links\s+WHERE id = \$1`).
		WithArgs(linkID).
		WillReturnRows(linkRows)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=section%20search&scope=section&limit=3", nil)
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

	if len(response.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(response.Results))
	}

	if response.Results[0].Type != "post" || response.Results[0].Post == nil || response.Results[0].Post.ID != postID {
		t.Fatalf("expected first result to be post %s", postID)
	}
	if response.Results[1].Type != "comment" || response.Results[1].Comment == nil || response.Results[1].Comment.ID != commentID {
		t.Fatalf("expected second result to be comment %s", commentID)
	}
	if response.Results[1].Post == nil || response.Results[1].Post.ID != postID {
		t.Fatalf("expected comment result to include post %s", postID)
	}
	if response.Results[2].Type != "link_metadata" || response.Results[2].LinkMetadata == nil || response.Results[2].LinkMetadata.ID != linkID {
		t.Fatalf("expected third result to be link metadata %s", linkID)
	}

	if response.Results[0].Score < response.Results[1].Score || response.Results[1].Score < response.Results[2].Score {
		t.Fatalf("expected scores to be in descending order")
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
	limit := 3
	postID := uuid.New()
	commentID := uuid.New()
	linkID := uuid.New()
	userID := uuid.New()
	sectionID := uuid.New()
	postCreated := time.Now()
	commentCreated := time.Now()
	userCreated := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT plainto_tsquery('english', $1)::text")).
		WithArgs(query).
		WillReturnRows(sqlmock.NewRows([]string{"plainto_tsquery"}).AddRow("hello & world"))

	searchRows := sqlmock.NewRows([]string{"result_type", "id", "rank"}).
		AddRow("post", postID, 0.42).
		AddRow("comment", commentID, 0.36).
		AddRow("link_metadata", linkID, 0.31)

	mock.ExpectQuery(regexp.QuoteMeta("WITH q AS")).
		WithArgs(query, limit).
		WillReturnRows(searchRows)

	postRows := sqlmock.NewRows([]string{
		"id", "user_id", "section_id", "content", "created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at", "comment_count", "type",
	}).AddRow(
		postID, userID, sectionID, "post content", postCreated, nil, nil, nil,
		userID, "alice", "alice@example.com", nil, nil, false, userCreated, 0, "general",
	)

	mock.ExpectQuery(regexp.QuoteMeta("FROM posts p")).
		WithArgs(postID).
		WillReturnRows(postRows)

	mock.ExpectQuery(regexp.QuoteMeta("FROM links")).
		WithArgs(postID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url", "metadata", "created_at"}))

	mock.ExpectQuery(regexp.QuoteMeta("FROM post_images")).
		WithArgs(postID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "image_url", "position", "caption", "alt_text", "created_at"}))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT emoji, COUNT")).
		WithArgs(postID).
		WillReturnRows(sqlmock.NewRows([]string{"emoji", "count"}))

	commentRows := sqlmock.NewRows([]string{
		"id", "user_id", "post_id", "section_id", "parent_comment_id", "image_id", "timestamp_seconds", "content", "contains_spoiler", "created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at",
	}).AddRow(
		commentID, userID, postID, sectionID, nil, nil, nil, "comment content", false, commentCreated, nil, nil, nil,
		userID, "alice", "alice@example.com", nil, nil, false, userCreated,
	)

	mock.ExpectQuery(regexp.QuoteMeta("FROM comments c")).
		WithArgs(commentID).
		WillReturnRows(commentRows)

	mock.ExpectQuery(regexp.QuoteMeta("FROM links")).
		WithArgs(commentID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url", "metadata", "created_at"}))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT emoji, COUNT")).
		WithArgs(commentID).
		WillReturnRows(sqlmock.NewRows([]string{"emoji", "count"}))

	linkRows := sqlmock.NewRows([]string{"id", "url", "metadata", "post_id", "comment_id"}).
		AddRow(linkID, "https://example.com", []byte(`{"title":"Example"}`), postID, nil)

	mock.ExpectQuery(`FROM links\s+WHERE id = \$1`).
		WithArgs(linkID).
		WillReturnRows(linkRows)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=hello%20world&scope=global&limit=3", nil)
	rr := httptest.NewRecorder()

	handler.Search(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}

	var response models.SearchResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(response.Results))
	}

	if response.Results[0].Type != "post" || response.Results[0].Post == nil || response.Results[0].Post.ID != postID {
		t.Fatalf("expected first result to be post %s", postID)
	}
	if response.Results[1].Type != "comment" || response.Results[1].Comment == nil || response.Results[1].Comment.ID != commentID {
		t.Fatalf("expected second result to be comment %s", commentID)
	}
	if response.Results[1].Post == nil || response.Results[1].Post.ID != postID {
		t.Fatalf("expected comment result to include post %s", postID)
	}
	if response.Results[2].Type != "link_metadata" || response.Results[2].LinkMetadata == nil || response.Results[2].LinkMetadata.ID != linkID {
		t.Fatalf("expected third result to be link metadata %s", linkID)
	}

	if response.Results[0].Score < response.Results[1].Score || response.Results[1].Score < response.Results[2].Score {
		t.Fatalf("expected scores to be in descending order")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}

}
