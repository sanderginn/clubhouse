package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
)

func TestCreateCommentHandlerMethodNotAllowed(t *testing.T) {
	// Mock handler
	handler := &CommentHandler{}

	// Test GET method should be rejected
	req, err := http.NewRequest("GET", "/api/v1/comments", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler.CreateComment(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusMethodNotAllowed)
	}

	var errResp models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatal(err)
	}

	if errResp.Code != "METHOD_NOT_ALLOWED" {
		t.Errorf("handler returned wrong error code: got %v want METHOD_NOT_ALLOWED", errResp.Code)
	}
}

func TestCreateCommentHandlerInvalidRequest(t *testing.T) {
	handler := &CommentHandler{}

	// Test invalid JSON
	req, err := http.NewRequest("POST", "/api/v1/comments", bytes.NewReader([]byte("invalid json")))
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(createTestUserContext(req.Context(), uuid.New(), "testuser", false))

	rr := httptest.NewRecorder()
	handler.CreateComment(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	var errResp models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatal(err)
	}

	if errResp.Code != "INVALID_REQUEST" {
		t.Errorf("handler returned wrong error code: got %v want INVALID_REQUEST", errResp.Code)
	}
}

func TestCreateCommentHandlerMissingUserID(t *testing.T) {
	handler := &CommentHandler{}

	reqBody := models.CreateCommentRequest{
		PostID:  uuid.New().String(),
		Content: "Test comment",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/api/v1/comments", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler.CreateComment(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusUnauthorized)
	}

	var errResp models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatal(err)
	}

	if errResp.Code != "UNAUTHORIZED" {
		t.Errorf("handler returned wrong error code: got %v want UNAUTHORIZED", errResp.Code)
	}
}

func TestCreateCommentHandlerRateLimited(t *testing.T) {
	limiter := &stubContentRateLimiter{allowed: false}
	handler := &CommentHandler{rateLimiter: limiter}

	reqBody := models.CreateCommentRequest{
		PostID:  uuid.New().String(),
		Content: "Test comment",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatal(err)
	}

	userID := uuid.New()
	req, err := http.NewRequest("POST", "/api/v1/comments", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(createTestUserContext(req.Context(), userID, "testuser", false))

	rr := httptest.NewRecorder()
	handler.CreateComment(rr, req)

	if status := rr.Code; status != http.StatusTooManyRequests {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusTooManyRequests)
	}

	if !limiter.called {
		t.Fatalf("expected rate limiter to be called")
	}
	if limiter.key != userID.String() {
		t.Fatalf("expected rate limiter key %s, got %s", userID.String(), limiter.key)
	}

	var errResp models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatal(err)
	}

	if errResp.Code != "RATE_LIMITED" {
		t.Errorf("handler returned wrong error code: got %v want RATE_LIMITED", errResp.Code)
	}
}

func TestCreateCommentHandlerRateLimitAllowsInvalidBody(t *testing.T) {
	limiter := &stubContentRateLimiter{allowed: true}
	handler := &CommentHandler{rateLimiter: limiter}

	userID := uuid.New()
	req, err := http.NewRequest("POST", "/api/v1/comments", bytes.NewReader([]byte("{")))
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(createTestUserContext(req.Context(), userID, "testuser", false))

	rr := httptest.NewRecorder()
	handler.CreateComment(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}

	if !limiter.called {
		t.Fatalf("expected rate limiter to be called")
	}
	if limiter.key != userID.String() {
		t.Fatalf("expected rate limiter key %s, got %s", userID.String(), limiter.key)
	}

	var errResp models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatal(err)
	}

	if errResp.Code != "INVALID_REQUEST" {
		t.Errorf("handler returned wrong error code: got %v want INVALID_REQUEST", errResp.Code)
	}
}

func TestCreateCommentHandlerRequestTooLarge(t *testing.T) {
	handler := &CommentHandler{}

	largeContent := strings.Repeat("a", int(maxJSONBodyBytes)+1024)
	reqBody := models.CreateCommentRequest{
		PostID:  uuid.New().String(),
		Content: largeContent,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/api/v1/comments", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(createTestUserContext(req.Context(), uuid.New(), "testuser", false))

	rr := httptest.NewRecorder()
	handler.CreateComment(rr, req)

	if status := rr.Code; status != http.StatusRequestEntityTooLarge {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusRequestEntityTooLarge)
	}

	var errResp models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatal(err)
	}

	if errResp.Code != "REQUEST_TOO_LARGE" {
		t.Errorf("handler returned wrong error code: got %v want REQUEST_TOO_LARGE", errResp.Code)
	}
}

func TestCreateCommentHandlerInvalidImageID(t *testing.T) {
	db, mock, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("failed to setup mock db: %v", err)
	}
	defer db.Close()

	handler := NewCommentHandler(db, nil, nil)
	handler.rateLimiter = &stubContentRateLimiter{allowed: true}

	userID := uuid.New()
	postID := uuid.New()
	sectionID := uuid.New()

	body, err := json.Marshal(models.CreateCommentRequest{
		PostID:  postID.String(),
		Content: "Comment with invalid image id",
		ImageID: stringPtr("not-a-uuid"),
	})
	if err != nil {
		t.Fatalf("failed to marshal body: %v", err)
	}

	mock.ExpectQuery("SELECT p.section_id, s.name, s.type FROM posts").
		WithArgs(postID).
		WillReturnRows(sqlmock.NewRows([]string{"section_id", "name", "type"}).AddRow(sectionID, "General", "general"))

	req, err := http.NewRequest(http.MethodPost, "/api/v1/comments", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req = req.WithContext(createTestUserContext(req.Context(), userID, "testuser", false))

	rr := httptest.NewRecorder()
	handler.CreateComment(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Fatalf("expected status %v, got %v", http.StatusBadRequest, status)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Code != "INVALID_IMAGE_ID" {
		t.Fatalf("expected code INVALID_IMAGE_ID, got %s", response.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}

func TestCreateCommentHandlerImageNotFound(t *testing.T) {
	db, mock, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("failed to setup mock db: %v", err)
	}
	defer db.Close()

	handler := NewCommentHandler(db, nil, nil)
	handler.rateLimiter = &stubContentRateLimiter{allowed: true}

	userID := uuid.New()
	postID := uuid.New()
	sectionID := uuid.New()
	imageID := uuid.New()

	body, err := json.Marshal(models.CreateCommentRequest{
		PostID:  postID.String(),
		Content: "Comment with missing image",
		ImageID: stringPtr(imageID.String()),
	})
	if err != nil {
		t.Fatalf("failed to marshal body: %v", err)
	}

	mock.ExpectQuery("SELECT p.section_id, s.name, s.type FROM posts").
		WithArgs(postID).
		WillReturnRows(sqlmock.NewRows([]string{"section_id", "name", "type"}).AddRow(sectionID, "General", "general"))
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM post_images").
		WithArgs(imageID, postID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	req, err := http.NewRequest(http.MethodPost, "/api/v1/comments", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req = req.WithContext(createTestUserContext(req.Context(), userID, "testuser", false))

	rr := httptest.NewRecorder()
	handler.CreateComment(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Fatalf("expected status %v, got %v", http.StatusNotFound, status)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Code != "IMAGE_NOT_FOUND" {
		t.Fatalf("expected code IMAGE_NOT_FOUND, got %s", response.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}

func TestGetCommentHandlerMethodNotAllowed(t *testing.T) {
	handler := &CommentHandler{}

	// Test POST method should be rejected
	req, err := http.NewRequest("POST", "/api/v1/comments/"+uuid.New().String(), nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler.GetComment(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusMethodNotAllowed)
	}

	var errResp models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatal(err)
	}

	if errResp.Code != "METHOD_NOT_ALLOWED" {
		t.Errorf("handler returned wrong error code: got %v want METHOD_NOT_ALLOWED", errResp.Code)
	}
}

func TestGetCommentHandlerInvalidID(t *testing.T) {
	handler := &CommentHandler{}

	req, err := http.NewRequest("GET", "/api/v1/comments/invalid-uuid", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler.GetComment(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	var errResp models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatal(err)
	}

	if errResp.Code != "INVALID_COMMENT_ID" {
		t.Errorf("handler returned wrong error code: got %v want INVALID_COMMENT_ID", errResp.Code)
	}
}

func TestDeleteCommentHandlerMethodNotAllowed(t *testing.T) {
	handler := &CommentHandler{}

	req, err := http.NewRequest("GET", "/api/v1/comments/"+uuid.New().String(), nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler.DeleteComment(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusMethodNotAllowed)
	}

	var errResp models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatal(err)
	}

	if errResp.Code != "METHOD_NOT_ALLOWED" {
		t.Errorf("handler returned wrong error code: got %v want METHOD_NOT_ALLOWED", errResp.Code)
	}
}

func TestDeleteCommentHandlerMissingUserID(t *testing.T) {
	handler := &CommentHandler{}

	req, err := http.NewRequest("DELETE", "/api/v1/comments/"+uuid.New().String(), nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler.DeleteComment(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusUnauthorized)
	}

	var errResp models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatal(err)
	}

	if errResp.Code != "UNAUTHORIZED" {
		t.Errorf("handler returned wrong error code: got %v want UNAUTHORIZED", errResp.Code)
	}
}

func TestUpdateCommentSuccess(t *testing.T) {
	db, mock, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("failed to setup mock db: %v", err)
	}
	defer db.Close()

	handler := NewCommentHandler(db, nil, nil)
	userID := uuid.New()
	commentID := uuid.New()
	postID := uuid.New()
	sectionID := uuid.New()
	now := time.Now()
	updatedAt := now.Add(time.Minute)

	body, err := json.Marshal(models.UpdateCommentRequest{Content: "Updated comment"})
	if err != nil {
		t.Fatalf("failed to marshal body: %v", err)
	}

	mock.ExpectQuery("SELECT c.user_id, c.content, c.post_id, p.section_id, s.type").WithArgs(commentID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "content", "post_id", "section_id", "type"}).AddRow(userID, "Original comment", postID, sectionID, "general"))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE comments").WithArgs("Updated comment", commentID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO audit_logs").WithArgs(userID, "update_comment", userID, userID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	rows := mock.NewRows([]string{
		"id", "user_id", "post_id", "section_id", "parent_comment_id", "image_id", "timestamp_seconds", "content",
		"created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at",
	}).AddRow(
		commentID, userID, postID, sectionID, nil, nil, nil, "Updated comment",
		now, updatedAt, nil, nil,
		userID, "testuser", "test@example.com", nil, nil, false, now,
	)
	mock.ExpectQuery("SELECT").WithArgs(commentID).WillReturnRows(rows)

	linksRows := mock.NewRows([]string{"id", "url", "metadata", "created_at"})
	mock.ExpectQuery("SELECT id, url, metadata, created_at").WithArgs(commentID).WillReturnRows(linksRows)

	reactionRows := mock.NewRows([]string{"emoji", "count"})
	mock.ExpectQuery("SELECT emoji, COUNT").WithArgs(commentID).WillReturnRows(reactionRows)

	viewerRows := mock.NewRows([]string{"emoji"})
	mock.ExpectQuery("SELECT emoji").WithArgs(commentID, userID).WillReturnRows(viewerRows)

	req, err := http.NewRequest(http.MethodPatch, "/api/v1/comments/"+commentID.String(), bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req = req.WithContext(createTestUserContext(req.Context(), userID, "testuser", false))

	rr := httptest.NewRecorder()
	handler.UpdateComment(rr, req)

	if status := rr.Code; status != http.StatusOK {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("expected status %v, got %v (mock: %v)", http.StatusOK, status, err)
		}
		t.Fatalf("expected status %v, got %v", http.StatusOK, status)
	}

	var response models.UpdateCommentResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Comment.Content != "Updated comment" {
		t.Fatalf("expected content to be updated, got %s", response.Comment.Content)
	}

	if response.Comment.UpdatedAt == nil {
		t.Fatal("expected updated_at to be set")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}

}

func TestUpdateCommentEmptyContent(t *testing.T) {
	handler := &CommentHandler{commentService: services.NewCommentService(nil)}
	commentID := uuid.New()

	body, err := json.Marshal(models.UpdateCommentRequest{Content: "   "})
	if err != nil {
		t.Fatalf("failed to marshal body: %v", err)
	}

	req, err := http.NewRequest(http.MethodPatch, "/api/v1/comments/"+commentID.String(), bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req = req.WithContext(createTestUserContext(req.Context(), uuid.New(), "testuser", false))

	rr := httptest.NewRecorder()
	handler.UpdateComment(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Fatalf("expected status %v, got %v", http.StatusBadRequest, status)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Code != "CONTENT_REQUIRED" {
		t.Fatalf("expected code CONTENT_REQUIRED, got %s", response.Code)
	}
}

func TestUpdateCommentForbidden(t *testing.T) {
	db, mock, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("failed to setup mock db: %v", err)
	}
	defer db.Close()

	handler := NewCommentHandler(db, nil, nil)
	userID := uuid.New()
	commentID := uuid.New()

	body, err := json.Marshal(models.UpdateCommentRequest{Content: "Updated comment"})
	if err != nil {
		t.Fatalf("failed to marshal body: %v", err)
	}

	mock.ExpectQuery("SELECT c.user_id, c.content, c.post_id, p.section_id, s.type").
		WithArgs(commentID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "content", "post_id", "section_id", "type"}).AddRow(uuid.New(), "Original comment", uuid.New(), uuid.New(), "general"))

	req, err := http.NewRequest(http.MethodPatch, "/api/v1/comments/"+commentID.String(), bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req = req.WithContext(createTestUserContext(req.Context(), userID, "testuser", false))

	rr := httptest.NewRecorder()
	handler.UpdateComment(rr, req)

	if status := rr.Code; status != http.StatusForbidden {
		t.Fatalf("expected status %v, got %v", http.StatusForbidden, status)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Code != "FORBIDDEN" {
		t.Fatalf("expected code FORBIDDEN, got %s", response.Code)
	}

}

func TestDeleteCommentHandlerInvalidID(t *testing.T) {
	handler := &CommentHandler{}

	req, err := http.NewRequest("DELETE", "/api/v1/comments/invalid-uuid", nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.WithValue(req.Context(), middleware.UserContextKey, &services.Session{
		ID:       uuid.New().String(),
		UserID:   uuid.New(),
		Username: "testuser",
		IsAdmin:  false,
	})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.DeleteComment(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	var errResp models.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatal(err)
	}

	if errResp.Code != "INVALID_COMMENT_ID" {
		t.Errorf("handler returned wrong error code: got %v want INVALID_COMMENT_ID", errResp.Code)
	}
}

func stringPtr(value string) *string {
	return &value
}
