package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
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
