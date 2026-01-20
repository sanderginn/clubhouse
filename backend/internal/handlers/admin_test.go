package handlers

import (
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
	if db == nil {
		t.Skip("test database not configured")
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
	if db == nil {
		t.Skip("test database not configured")
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
	if db == nil {
		t.Skip("test database not configured")
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
	if db == nil {
		t.Skip("test database not configured")
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

// TestHardDeletePost tests permanently deleting a post
func TestHardDeletePost(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

	// Create a test admin user
	adminID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'testadmin', 'testadmin@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	// Create a test section
	sectionID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO sections (id, name, slug, description, created_at)
		VALUES ($1, 'Test Section', 'test-section', 'A test section', now())
	`, sectionID)
	if err != nil {
		t.Fatalf("failed to create test section: %v", err)
	}

	// Create a test post
	postID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO posts (id, user_id, section_id, content, created_at)
		VALUES ($1, $2, $3, 'Test post content', now())
	`, postID, adminID, sectionID)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	handler := NewAdminHandler(db)

	// Test hard delete request
	req := httptest.NewRequest("DELETE", "/api/v1/admin/posts/"+postID.String(), nil)
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "testadmin", true))
	w := httptest.NewRecorder()

	handler.HardDeletePost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.HardDeletePostResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.ID != postID {
		t.Errorf("expected post ID %s, got %s", postID, response.ID)
	}

	// Verify post is deleted from DB
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM posts WHERE id = $1", postID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query post count: %v", err)
	}

	if count != 0 {
		t.Errorf("expected post to be deleted, but found %d posts", count)
	}

	// Verify audit log was created (query by admin_user_id since related_post_id becomes NULL after ON DELETE SET NULL)
	var auditCount int
	err = db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action = 'hard_delete_post' AND admin_user_id = $1", adminID).Scan(&auditCount)
	if err != nil {
		t.Fatalf("failed to query audit log count: %v", err)
	}

	if auditCount != 1 {
		t.Errorf("expected 1 audit log entry, but found %d", auditCount)
	}
}

// TestHardDeleteComment tests permanently deleting a comment
func TestHardDeleteComment(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

	// Create a test admin user
	adminID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'testadmin2', 'testadmin2@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	// Create a test section
	sectionID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO sections (id, name, slug, description, created_at)
		VALUES ($1, 'Test Section 2', 'test-section-2', 'A test section', now())
	`, sectionID)
	if err != nil {
		t.Fatalf("failed to create test section: %v", err)
	}

	// Create a test post
	postID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO posts (id, user_id, section_id, content, created_at)
		VALUES ($1, $2, $3, 'Test post content', now())
	`, postID, adminID, sectionID)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Create a test comment
	commentID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO comments (id, user_id, post_id, content, created_at)
		VALUES ($1, $2, $3, 'Test comment content', now())
	`, commentID, adminID, postID)
	if err != nil {
		t.Fatalf("failed to create test comment: %v", err)
	}

	handler := NewAdminHandler(db)

	// Test hard delete request
	req := httptest.NewRequest("DELETE", "/api/v1/admin/comments/"+commentID.String(), nil)
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "testadmin2", true))
	w := httptest.NewRecorder()

	handler.HardDeleteComment(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.HardDeleteCommentResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.ID != commentID {
		t.Errorf("expected comment ID %s, got %s", commentID, response.ID)
	}

	// Verify comment is deleted from DB
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM comments WHERE id = $1", commentID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query comment count: %v", err)
	}

	if count != 0 {
		t.Errorf("expected comment to be deleted, but found %d comments", count)
	}

	// Verify audit log was created (query by admin_user_id since related_comment_id becomes NULL after ON DELETE SET NULL)
	var auditCount int
	err = db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action = 'hard_delete_comment' AND admin_user_id = $1", adminID).Scan(&auditCount)
	if err != nil {
		t.Fatalf("failed to query audit log count: %v", err)
	}

	if auditCount != 1 {
		t.Errorf("expected 1 audit log entry, but found %d", auditCount)
	}
}

// TestHardDeletePostNotFound tests hard delete with invalid post ID
func TestHardDeletePostNotFound(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

	// Create a test admin user for context
	adminID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'testadmin3', 'testadmin3@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	handler := NewAdminHandler(db)

	// Test hard delete with non-existent post
	nonExistentID := uuid.New()
	req := httptest.NewRequest("DELETE", "/api/v1/admin/posts/"+nonExistentID.String(), nil)
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "testadmin3", true))
	w := httptest.NewRecorder()

	handler.HardDeletePost(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

// TestHardDeleteCommentNotFound tests hard delete with invalid comment ID
func TestHardDeleteCommentNotFound(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test DB: %v", err)
	}
	if db == nil {
		t.Skip("test database not configured")
	}
	defer db.Close()

	// Create a test admin user for context
	adminID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'testadmin4', 'testadmin4@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	handler := NewAdminHandler(db)

	// Test hard delete with non-existent comment
	nonExistentID := uuid.New()
	req := httptest.NewRequest("DELETE", "/api/v1/admin/comments/"+nonExistentID.String(), nil)
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "testadmin4", true))
	w := httptest.NewRecorder()

	handler.HardDeleteComment(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

// Helper function to get test database connection
func getTestDB() (*sql.DB, error) {
	// This would need proper test database setup
	// For now, return error to indicate test setup needed
	return nil, nil
}
