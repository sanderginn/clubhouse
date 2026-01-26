package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
