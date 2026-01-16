package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
)

// TestListPendingUsers tests listing users pending approval
func TestListPendingUsers(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	defer db.Close()

	handler := NewAdminHandler(db)

	req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	w := httptest.NewRecorder()

	handler.ListPendingUsers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var pendingUsers []*models.PendingUser
	if err := json.NewDecoder(w.Body).Decode(&pendingUsers); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if pendingUsers == nil {
		t.Errorf("expected non-nil pending users list")
	}
}

// TestApproveUser tests approving a pending user
func TestApproveUser(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	defer db.Close()

	// Create a test user
	userID := uuid.New()
	testUsername := "testuser"
	testEmail := "test@example.com"
	testHash := "$2a$12$test"

	query := `
		INSERT INTO users (id, username, email, password_hash, is_admin, created_at)
		VALUES ($1, $2, $3, $4, false, now())
	`
	_, err = db.Exec(query, userID, testUsername, testEmail, testHash)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	handler := NewAdminHandler(db)

	// Test approve request
	req := httptest.NewRequest("PATCH", "/api/v1/admin/users/"+userID.String()+"/approve", nil)
	w := httptest.NewRecorder()

	handler.ApproveUser(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.ApproveUserResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.ID != userID {
		t.Errorf("expected user ID %s, got %s", userID, response.ID)
	}

	// Verify user is approved in DB
	var approvedAt sql.NullTime
	err = db.QueryRow("SELECT approved_at FROM users WHERE id = $1", userID).Scan(&approvedAt)
	if err != nil {
		t.Fatalf("failed to query user: %v", err)
	}

	if !approvedAt.Valid {
		t.Errorf("expected approved_at to be set")
	}
}

// TestRejectUser tests rejecting a pending user
func TestRejectUser(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	defer db.Close()

	// Create a test user
	userID := uuid.New()
	testUsername := "rejectuser"
	testEmail := "reject@example.com"
	testHash := "$2a$12$test"

	query := `
		INSERT INTO users (id, username, email, password_hash, is_admin, created_at)
		VALUES ($1, $2, $3, $4, false, now())
	`
	_, err = db.Exec(query, userID, testUsername, testEmail, testHash)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	handler := NewAdminHandler(db)

	// Test reject request
	req := httptest.NewRequest("DELETE", "/api/v1/admin/users/"+userID.String(), nil)
	w := httptest.NewRecorder()

	handler.RejectUser(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.RejectUserResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.ID != userID {
		t.Errorf("expected user ID %s, got %s", userID, response.ID)
	}

	// Verify user is deleted from DB
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users WHERE id = $1", userID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query user count: %v", err)
	}

	if count != 0 {
		t.Errorf("expected user to be deleted, but found %d users", count)
	}
}

// TestApproveAlreadyApprovedUser tests error when approving already approved user
func TestApproveAlreadyApprovedUser(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	defer db.Close()

	// Create and approve a test user
	userID := uuid.New()
	testUsername := "approveduser"
	testEmail := "approved@example.com"
	testHash := "$2a$12$test"

	query := `
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, $2, $3, $4, false, now(), now())
	`
	_, err = db.Exec(query, userID, testUsername, testEmail, testHash)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	handler := NewAdminHandler(db)

	// Test approve request on already approved user
	req := httptest.NewRequest("PATCH", "/api/v1/admin/users/"+userID.String()+"/approve", nil)
	w := httptest.NewRecorder()

	handler.ApproveUser(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusConflict, w.Code, w.Body.String())
	}
}

// Helper function to get test database connection
func getTestDB() (*sql.DB, error) {
	// This would need proper test database setup
	// For now, return error to indicate test setup needed
	return nil, nil
}
