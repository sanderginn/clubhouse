package handlers

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

// TestListPendingUsers tests listing users pending approval
func TestListPendingUsers(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewAdminHandler(db, nil)

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

	// Empty database returns null which decodes to nil slice - that's acceptable
	if pendingUsers == nil {
		pendingUsers = []*models.PendingUser{}
	}

	// At minimum, confirm the response decoded successfully (test passed if we got here)
}

// TestListApprovedUsers tests listing approved users
func TestListApprovedUsers(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	approvedID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'approveduser', 'approved@example.com', '$2a$12$test', false, now(), now())
	`, approvedID)
	if err != nil {
		t.Fatalf("failed to create approved user: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, created_at)
		VALUES ($1, 'pendinguser', 'pending@example.com', '$2a$12$test', false, now())
	`, uuid.New())
	if err != nil {
		t.Fatalf("failed to create pending user: %v", err)
	}

	handler := NewAdminHandler(db, nil)

	req := httptest.NewRequest("GET", "/api/v1/admin/users/approved", nil)
	w := httptest.NewRecorder()

	handler.ListApprovedUsers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var approvedUsers []*models.ApprovedUser
	if err := json.NewDecoder(w.Body).Decode(&approvedUsers); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if len(approvedUsers) != 1 {
		t.Fatalf("expected 1 approved user, got %d", len(approvedUsers))
	}

	if approvedUsers[0].ID != approvedID {
		t.Errorf("expected user ID %s, got %s", approvedID, approvedUsers[0].ID)
	}
	if approvedUsers[0].ApprovedAt.IsZero() {
		t.Errorf("expected approved_at to be set")
	}
}

// TestApproveUser tests approving a pending user
func TestApproveUser(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create a test admin user
	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'approveadmin', 'approveadmin@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	// Create a test user to approve
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

	notificationID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO notifications (id, user_id, type, related_user_id, created_at)
		VALUES ($1, $2, 'user_registration_pending', $3, now())
	`, notificationID, adminID, userID)
	if err != nil {
		t.Fatalf("failed to create registration notification: %v", err)
	}

	handler := NewAdminHandler(db, nil)

	// Test approve request
	req := httptest.NewRequest("PATCH", "/api/v1/admin/users/"+userID.String()+"/approve", nil)
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "approveadmin", true))
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

	// Verify audit log was created
	var auditCount int
	err = db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action = 'approve_user' AND admin_user_id = $1 AND related_user_id = $2", adminID, userID).Scan(&auditCount)
	if err != nil {
		t.Fatalf("failed to query audit log count: %v", err)
	}

	if auditCount != 1 {
		t.Errorf("expected 1 audit log entry, but found %d", auditCount)
	}

	var readAt sql.NullTime
	err = db.QueryRow("SELECT read_at FROM notifications WHERE id = $1", notificationID).Scan(&readAt)
	if err != nil {
		t.Fatalf("failed to query notification: %v", err)
	}
	if !readAt.Valid {
		t.Errorf("expected registration notification to be marked read")
	}
}

// TestPromoteUser tests promoting a user to admin
func TestPromoteUser(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})

	// Create a test admin user
	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'promoteadmin', 'promoteadmin@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	// Create a user to promote
	userID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'memberuser', 'member@example.com', '$2a$12$test', false, now(), now())
	`, userID)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	sessionService := services.NewSessionService(redisClient)
	session, err := sessionService.CreateSession(t.Context(), userID, "memberuser", false)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	handler := NewAdminHandler(db, redisClient)

	req := httptest.NewRequest("POST", "/api/v1/admin/users/"+userID.String()+"/promote", nil)
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "promoteadmin", true))
	w := httptest.NewRecorder()

	handler.PromoteUser(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.PromoteUserResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.IsAdmin {
		t.Fatalf("expected promoted user to be admin")
	}

	var isAdmin bool
	if err := db.QueryRow("SELECT is_admin FROM users WHERE id = $1", userID).Scan(&isAdmin); err != nil {
		t.Fatalf("failed to query user: %v", err)
	}
	if !isAdmin {
		t.Fatalf("expected user to be admin in database")
	}

	updatedSession, err := sessionService.GetSession(t.Context(), session.ID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if !updatedSession.IsAdmin {
		t.Fatalf("expected session to be updated to admin")
	}

	var auditCount int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM audit_logs WHERE action = 'promote_to_admin' AND admin_user_id = $1 AND related_user_id = $2",
		adminID,
		userID,
	).Scan(&auditCount)
	if err != nil {
		t.Fatalf("failed to query audit log count: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 audit log entry, but found %d", auditCount)
	}
}

func TestPromoteUserCannotPromoteSelf(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'selfadmin', 'selfadmin@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	handler := NewAdminHandler(db, nil)

	req := httptest.NewRequest("POST", "/api/v1/admin/users/"+adminID.String()+"/promote", nil)
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "selfadmin", true))
	w := httptest.NewRecorder()

	handler.PromoteUser(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusForbidden, w.Code, w.Body.String())
	}
}

func TestSuspendUser(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})

	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'suspendadmin', 'suspendadmin@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	userID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'suspenduser', 'suspenduser@example.com', '$2a$12$test', false, now(), now())
	`, userID)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	sessionService := services.NewSessionService(redisClient)
	session, err := sessionService.CreateSession(t.Context(), userID, "suspenduser", false)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	handler := NewAdminHandler(db, redisClient)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+userID.String()+"/suspend", strings.NewReader(`{"reason":"spam"}`))
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "suspendadmin", true))
	w := httptest.NewRecorder()

	handler.SuspendUser(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.SuspendUserResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.ID != userID {
		t.Fatalf("expected user ID %s, got %s", userID, response.ID)
	}
	if response.SuspendedAt.IsZero() {
		t.Fatalf("expected suspended_at to be set")
	}

	var suspendedAt sql.NullTime
	if err := db.QueryRow("SELECT suspended_at FROM users WHERE id = $1", userID).Scan(&suspendedAt); err != nil {
		t.Fatalf("failed to query user: %v", err)
	}
	if !suspendedAt.Valid {
		t.Fatalf("expected suspended_at to be set")
	}

	var auditMetadata []byte
	err = db.QueryRow(`
		SELECT metadata
		FROM audit_logs
		WHERE action = 'suspend_user' AND admin_user_id = $1 AND related_user_id = $2
	`, adminID, userID).Scan(&auditMetadata)
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(auditMetadata, &metadata); err != nil {
		t.Fatalf("failed to unmarshal audit metadata: %v", err)
	}
	if metadata["reason"] != "spam" {
		t.Fatalf("expected reason metadata to be %q, got %v", "spam", metadata["reason"])
	}

	if _, err := sessionService.GetSession(t.Context(), session.ID); err == nil {
		t.Fatalf("expected session to be revoked")
	}
}

func TestUnsuspendUser(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'unsuspendadmin', 'unsuspendadmin@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	userID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, suspended_at, created_at)
		VALUES ($1, 'unsuspenduser', 'unsuspenduser@example.com', '$2a$12$test', false, now(), now(), now())
	`, userID)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	handler := NewAdminHandler(db, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+userID.String()+"/unsuspend", nil)
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "unsuspendadmin", true))
	w := httptest.NewRecorder()

	handler.UnsuspendUser(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.UnsuspendUserResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.ID != userID {
		t.Fatalf("expected user ID %s, got %s", userID, response.ID)
	}

	var suspendedAt sql.NullTime
	if err := db.QueryRow("SELECT suspended_at FROM users WHERE id = $1", userID).Scan(&suspendedAt); err != nil {
		t.Fatalf("failed to query user: %v", err)
	}
	if suspendedAt.Valid {
		t.Fatalf("expected suspended_at to be cleared")
	}

	var auditCount int
	err = db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action = 'unsuspend_user' AND admin_user_id = $1 AND related_user_id = $2", adminID, userID).Scan(&auditCount)
	if err != nil {
		t.Fatalf("failed to query audit log count: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 audit log entry, found %d", auditCount)
	}
}

// TestRejectUser tests rejecting a pending user
func TestRejectUser(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create a test admin user
	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'rejectadmin', 'rejectadmin@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	// Create a test user to reject
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

	notificationID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO notifications (id, user_id, type, related_user_id, created_at)
		VALUES ($1, $2, 'user_registration_pending', $3, now())
	`, notificationID, adminID, userID)
	if err != nil {
		t.Fatalf("failed to create registration notification: %v", err)
	}

	handler := NewAdminHandler(db, nil)

	// Test reject request
	req := httptest.NewRequest("DELETE", "/api/v1/admin/users/"+userID.String(), nil)
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "rejectadmin", true))
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

	// Verify audit log was created (related_user_id will be NULL due to ON DELETE SET NULL)
	var auditCount int
	err = db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action = 'reject_user' AND admin_user_id = $1", adminID).Scan(&auditCount)
	if err != nil {
		t.Fatalf("failed to query audit log count: %v", err)
	}

	if auditCount != 1 {
		t.Errorf("expected 1 audit log entry, but found %d", auditCount)
	}

	var readAt sql.NullTime
	err = db.QueryRow("SELECT read_at FROM notifications WHERE id = $1", notificationID).Scan(&readAt)
	if err != nil {
		t.Fatalf("failed to query notification: %v", err)
	}
	if !readAt.Valid {
		t.Errorf("expected registration notification to be marked read")
	}
}

// TestApproveAlreadyApprovedUser tests error when approving already approved user
func TestApproveAlreadyApprovedUser(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create a test admin user
	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'alreadyapprovedadmin', 'alreadyapprovedadmin@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

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

	handler := NewAdminHandler(db, nil)

	// Test approve request on already approved user
	req := httptest.NewRequest("PATCH", "/api/v1/admin/users/"+userID.String()+"/approve", nil)
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "alreadyapprovedadmin", true))
	w := httptest.NewRecorder()

	handler.ApproveUser(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusConflict, w.Code, w.Body.String())
	}
}

// TestHardDeletePost tests permanently deleting a post
func TestHardDeletePost(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create a test admin user
	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'testadmin', 'testadmin@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	// Create a test section
	sectionID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO sections (id, name, type, created_at)
		VALUES ($1, 'Test Section', 'general', now())
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

	handler := NewAdminHandler(db, nil)

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
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create a test admin user
	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'testadmin2', 'testadmin2@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	// Create a test section
	sectionID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO sections (id, name, type, created_at)
		VALUES ($1, 'Test Section 2', 'general', now())
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

	handler := NewAdminHandler(db, nil)

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
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create a test admin user for context
	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'testadmin3', 'testadmin3@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	handler := NewAdminHandler(db, nil)

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
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create a test admin user for context
	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'testadmin4', 'testadmin4@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	handler := NewAdminHandler(db, nil)

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

// TestAdminRestorePost tests restoring a soft-deleted post
func TestAdminRestorePost(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create a test admin user
	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'testadmin_restore', 'testadmin_restore@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	// Create a test section
	sectionID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO sections (id, name, type, created_at)
		VALUES ($1, 'Test Section Restore', 'general', now())
	`, sectionID)
	if err != nil {
		t.Fatalf("failed to create test section: %v", err)
	}

	// Create a soft-deleted post
	postID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO posts (id, user_id, section_id, content, created_at, deleted_at, deleted_by_user_id)
		VALUES ($1, $2, $3, 'Deleted post content', now(), now(), $2)
	`, postID, adminID, sectionID)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	handler := NewAdminHandler(db, nil)

	// Test restore request
	req := httptest.NewRequest("POST", "/api/v1/admin/posts/"+postID.String()+"/restore", nil)
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "testadmin_restore", true))
	w := httptest.NewRecorder()

	handler.AdminRestorePost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Verify post is restored in DB
	var deletedAt *string
	err = db.QueryRow("SELECT deleted_at FROM posts WHERE id = $1", postID).Scan(&deletedAt)
	if err != nil {
		t.Fatalf("failed to query post: %v", err)
	}

	if deletedAt != nil {
		t.Errorf("expected deleted_at to be NULL after restore")
	}

	// Verify audit log was created
	var auditCount int
	err = db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action = 'restore_post' AND admin_user_id = $1", adminID).Scan(&auditCount)
	if err != nil {
		t.Fatalf("failed to query audit log count: %v", err)
	}

	if auditCount != 1 {
		t.Errorf("expected 1 audit log entry, but found %d", auditCount)
	}
}

// TestAdminRestoreComment tests restoring a soft-deleted comment
func TestAdminRestoreComment(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create a test admin user
	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'testadmin_restore2', 'testadmin_restore2@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	// Create a test section
	sectionID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO sections (id, name, type, created_at)
		VALUES ($1, 'Test Section Restore 2', 'general', now())
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

	// Create a soft-deleted comment
	commentID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO comments (id, user_id, post_id, content, created_at, deleted_at, deleted_by_user_id)
		VALUES ($1, $2, $3, 'Deleted comment content', now(), now(), $2)
	`, commentID, adminID, postID)
	if err != nil {
		t.Fatalf("failed to create test comment: %v", err)
	}

	handler := NewAdminHandler(db, nil)

	// Test restore request
	req := httptest.NewRequest("POST", "/api/v1/admin/comments/"+commentID.String()+"/restore", nil)
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "testadmin_restore2", true))
	w := httptest.NewRecorder()

	handler.AdminRestoreComment(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Verify comment is restored in DB
	var deletedAt *string
	err = db.QueryRow("SELECT deleted_at FROM comments WHERE id = $1", commentID).Scan(&deletedAt)
	if err != nil {
		t.Fatalf("failed to query comment: %v", err)
	}

	if deletedAt != nil {
		t.Errorf("expected deleted_at to be NULL after restore")
	}

	// Verify audit log was created
	var auditCount int
	err = db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action = 'restore_comment' AND admin_user_id = $1", adminID).Scan(&auditCount)
	if err != nil {
		t.Fatalf("failed to query audit log count: %v", err)
	}

	if auditCount != 1 {
		t.Errorf("expected 1 audit log entry, but found %d", auditCount)
	}
}

// TestAdminRestorePostNotDeleted tests restore fails for non-deleted post
func TestAdminRestorePostNotDeleted(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create a test admin user
	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'testadmin_restore3', 'testadmin_restore3@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	// Create a test section
	sectionID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO sections (id, name, type, created_at)
		VALUES ($1, 'Test Section Restore 3', 'general', now())
	`, sectionID)
	if err != nil {
		t.Fatalf("failed to create test section: %v", err)
	}

	// Create a non-deleted post
	postID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO posts (id, user_id, section_id, content, created_at)
		VALUES ($1, $2, $3, 'Test post content', now())
	`, postID, adminID, sectionID)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	handler := NewAdminHandler(db, nil)

	// Test restore request on non-deleted post
	req := httptest.NewRequest("POST", "/api/v1/admin/posts/"+postID.String()+"/restore", nil)
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "testadmin_restore3", true))
	w := httptest.NewRecorder()

	handler.AdminRestorePost(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusConflict, w.Code, w.Body.String())
	}
}

// TestAdminRestorePostNotFound tests restore fails for non-existent post
func TestAdminRestorePostNotFound(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create a test admin user
	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'testadmin_restore4', 'testadmin_restore4@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	handler := NewAdminHandler(db, nil)

	// Test restore request on non-existent post
	nonExistentID := uuid.New()
	req := httptest.NewRequest("POST", "/api/v1/admin/posts/"+nonExistentID.String()+"/restore", nil)
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "testadmin_restore4", true))
	w := httptest.NewRecorder()

	handler.AdminRestorePost(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

// TestGetConfig tests getting the current config
func TestGetConfig(t *testing.T) {
	handler := NewAdminHandler(nil, nil) // No DB needed for config

	req := httptest.NewRequest("GET", "/api/v1/admin/config", nil)
	w := httptest.NewRecorder()

	handler.GetConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response struct {
		Config struct {
			LinkMetadataEnabled bool   `json:"linkMetadataEnabled"`
			DisplayTimezone     string `json:"displayTimezone"`
		} `json:"config"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	// Default should be enabled
	if !response.Config.LinkMetadataEnabled {
		t.Errorf("expected linkMetadataEnabled to be true by default")
	}
	if response.Config.DisplayTimezone != "UTC" {
		t.Errorf("expected displayTimezone to be UTC by default")
	}
}

// TestUpdateConfig tests updating the config
func TestUpdateConfig(t *testing.T) {
	handler := NewAdminHandler(nil, nil) // No DB needed for config

	// Test disabling link metadata and setting timezone
	body := `{"linkMetadataEnabled": false, "mfa_required": true, "display_timezone": "Europe/Amsterdam"}`
	req := httptest.NewRequest("PATCH", "/api/v1/admin/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response struct {
		Config struct {
			LinkMetadataEnabled bool   `json:"linkMetadataEnabled"`
			MFARequired         bool   `json:"mfaRequired"`
			DisplayTimezone     string `json:"displayTimezone"`
		} `json:"config"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Config.LinkMetadataEnabled {
		t.Errorf("expected linkMetadataEnabled to be false after update")
	}
	if !response.Config.MFARequired {
		t.Errorf("expected mfaRequired to be true after update")
	}
	if response.Config.DisplayTimezone != "Europe/Amsterdam" {
		t.Errorf("expected displayTimezone to be Europe/Amsterdam after update")
	}

	// Verify the change persists by getting config again
	req = httptest.NewRequest("GET", "/api/v1/admin/config", nil)
	w = httptest.NewRecorder()
	handler.GetConfig(w, req)

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Config.LinkMetadataEnabled {
		t.Errorf("expected linkMetadataEnabled to still be false")
	}

	// Test re-enabling link metadata and resetting timezone
	body = `{"linkMetadataEnabled": true, "mfa_required": false, "displayTimezone": "UTC"}`
	req = httptest.NewRequest("PATCH", "/api/v1/admin/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	handler.UpdateConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if !response.Config.LinkMetadataEnabled {
		t.Errorf("expected linkMetadataEnabled to be true after re-enabling")
	}
	if response.Config.MFARequired {
		t.Errorf("expected mfaRequired to be false after update")
	}
	if response.Config.DisplayTimezone != "UTC" {
		t.Errorf("expected displayTimezone to be UTC after update")
	}
}

func TestUpdateConfigInvalidTimezone(t *testing.T) {
	handler := NewAdminHandler(nil, nil)

	body := `{"display_timezone": "Not/AZone"}`
	req := httptest.NewRequest("PATCH", "/api/v1/admin/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestUpdateConfigAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() {
		testutil.CleanupTables(t, db)
		services.ResetConfigServiceForTests()
	})
	services.ResetConfigServiceForTests()

	adminID := uuid.MustParse(testutil.CreateTestUser(t, db, "configadmin", "configadmin@example.com", true, true))
	handler := NewAdminHandler(db, nil)

	configService := services.GetConfigService()
	current := configService.GetConfig().LinkMetadataEnabled
	t.Cleanup(func() {
		restore := current
		if _, err := configService.UpdateConfig(context.Background(), &restore, nil, nil); err != nil {
			t.Fatalf("failed to restore link metadata config: %v", err)
		}
	})

	next := !current
	body := fmt.Sprintf(`{"linkMetadataEnabled": %t}`, next)
	req := httptest.NewRequest("PATCH", "/api/v1/admin/config", strings.NewReader(body))
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "configadmin", true))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var metadataBytes []byte
	err := db.QueryRow(`
		SELECT metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'toggle_link_metadata'
		ORDER BY created_at DESC
		LIMIT 1
	`, adminID).Scan(&metadataBytes)
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if metadata["setting"] != "link_metadata_enabled" {
		t.Errorf("expected setting 'link_metadata_enabled', got %v", metadata["setting"])
	}
	if metadata["old_value"] != current {
		t.Errorf("expected old_value %v, got %v", current, metadata["old_value"])
	}
	if metadata["new_value"] != next {
		t.Errorf("expected new_value %v, got %v", next, metadata["new_value"])
	}
}

func TestUpdateConfigAuditLogMFARequired(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() {
		testutil.CleanupTables(t, db)
		services.ResetConfigServiceForTests()
	})
	services.ResetConfigServiceForTests()

	adminID := uuid.MustParse(testutil.CreateTestUser(t, db, "mfaconfigadmin", "mfaconfigadmin@example.com", true, true))
	handler := NewAdminHandler(db, nil)

	configService := services.GetConfigService()
	current := configService.GetConfig().MFARequired
	t.Cleanup(func() {
		restore := current
		if _, err := configService.UpdateConfig(context.Background(), nil, &restore, nil); err != nil {
			t.Fatalf("failed to restore mfa_required config: %v", err)
		}
	})

	next := !current
	body := fmt.Sprintf(`{"mfa_required": %t}`, next)
	req := httptest.NewRequest("PATCH", "/api/v1/admin/config", strings.NewReader(body))
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "mfaconfigadmin", true))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var metadataBytes []byte
	err := db.QueryRow(`
		SELECT metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'toggle_mfa_requirement'
		ORDER BY created_at DESC
		LIMIT 1
	`, adminID).Scan(&metadataBytes)
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if metadata["setting"] != "mfa_required" {
		t.Errorf("expected setting 'mfa_required', got %v", metadata["setting"])
	}
	if metadata["old_value"] != current {
		t.Errorf("expected old_value %v, got %v", current, metadata["old_value"])
	}
	if metadata["new_value"] != next {
		t.Errorf("expected new_value %v, got %v", next, metadata["new_value"])
	}
}

func TestUpdateConfigAuditLogDisplayTimezone(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() {
		testutil.CleanupTables(t, db)
		services.ResetConfigServiceForTests()
	})
	services.ResetConfigServiceForTests()

	adminID := uuid.MustParse(testutil.CreateTestUser(t, db, "tzadmin", "tzadmin@example.com", true, true))
	handler := NewAdminHandler(db, nil)

	configService := services.GetConfigService()
	current := configService.GetConfig().DisplayTimezone
	t.Cleanup(func() {
		restore := current
		if _, err := configService.UpdateConfig(context.Background(), nil, nil, &restore); err != nil {
			t.Fatalf("failed to restore display_timezone config: %v", err)
		}
	})

	next := "Europe/Amsterdam"
	if current == next {
		next = "UTC"
	}
	body := fmt.Sprintf(`{"display_timezone": "%s"}`, next)
	req := httptest.NewRequest("PATCH", "/api/v1/admin/config", strings.NewReader(body))
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "tzadmin", true))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var metadataBytes []byte
	err := db.QueryRow(`
		SELECT metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'update_display_timezone'
		ORDER BY created_at DESC
		LIMIT 1
	`, adminID).Scan(&metadataBytes)
	if err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if metadata["setting"] != "display_timezone" {
		t.Errorf("expected setting 'display_timezone', got %v", metadata["setting"])
	}
	if metadata["old_value"] != current {
		t.Errorf("expected old_value %v, got %v", current, metadata["old_value"])
	}
	if metadata["new_value"] != next {
		t.Errorf("expected new_value %v, got %v", next, metadata["new_value"])
	}
}

func TestUpdateConfigPersistsToDB(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() {
		testutil.CleanupTables(t, db)
		services.ResetConfigServiceForTests()
	})
	services.ResetConfigServiceForTests()

	if err := services.InitConfigService(context.Background(), db); err != nil {
		t.Fatalf("failed to init config service: %v", err)
	}

	handler := NewAdminHandler(db, nil)
	body := `{"linkMetadataEnabled": false, "mfa_required": true, "display_timezone": "America/New_York"}`
	req := httptest.NewRequest("PATCH", "/api/v1/admin/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var linkMetadataEnabled bool
	var mfaRequired bool
	var displayTimezone string
	err := db.QueryRow(`
		SELECT link_metadata_enabled, mfa_required, display_timezone
		FROM admin_config
		WHERE id = 1
	`).Scan(&linkMetadataEnabled, &mfaRequired, &displayTimezone)
	if err != nil {
		t.Fatalf("failed to query admin_config: %v", err)
	}
	if linkMetadataEnabled {
		t.Errorf("expected link_metadata_enabled false, got true")
	}
	if !mfaRequired {
		t.Errorf("expected mfa_required true, got false")
	}
	if displayTimezone != "America/New_York" {
		t.Errorf("expected display_timezone America/New_York, got %s", displayTimezone)
	}
}

// TestUpdateConfigMethodNotAllowed tests that GET to UpdateConfig is rejected
func TestUpdateConfigMethodNotAllowed(t *testing.T) {
	handler := NewAdminHandler(nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/admin/config", nil)
	w := httptest.NewRecorder()

	handler.UpdateConfig(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestGetConfigMethodNotAllowed tests that PATCH to GetConfig is rejected
func TestGetConfigMethodNotAllowed(t *testing.T) {
	handler := NewAdminHandler(nil, nil)

	req := httptest.NewRequest("PATCH", "/api/v1/admin/config", nil)
	w := httptest.NewRecorder()

	handler.GetConfig(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestUpdateConfigInvalidJSON tests that invalid JSON is rejected
func TestUpdateConfigInvalidJSON(t *testing.T) {
	handler := NewAdminHandler(nil, nil)

	body := `{invalid json}`
	req := httptest.NewRequest("PATCH", "/api/v1/admin/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestGetAuditLogs tests listing audit logs
func TestGetAuditLogs(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create a test admin user
	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'auditlogsadmin', 'auditlogsadmin@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	// Create a target user
	targetID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO users (id, username, email, password_hash, approved_at, created_at)
		VALUES ($1, 'audittarget', 'audittarget@example.com', '$2a$12$test', now(), now())
	`, targetID)
	if err != nil {
		t.Fatalf("failed to create target user: %v", err)
	}

	// Create some audit log entries
	_, err = db.Exec(`
		INSERT INTO audit_logs (id, admin_user_id, action, target_user_id, metadata, created_at)
		VALUES ($1, $2, 'test_action_1', $3, $4::jsonb, now())
	`, uuid.New(), adminID, targetID, `{"note":"hello"}`)
	if err != nil {
		t.Fatalf("failed to create audit log 1: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO audit_logs (id, admin_user_id, action, created_at)
		VALUES ($1, $2, 'test_action_2', now())
	`, uuid.New(), adminID)
	if err != nil {
		t.Fatalf("failed to create audit log 2: %v", err)
	}

	handler := NewAdminHandler(db, nil)

	req := httptest.NewRequest("GET", "/api/v1/admin/audit-logs", nil)
	w := httptest.NewRecorder()

	handler.GetAuditLogs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.AuditLogsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Logs == nil {
		t.Errorf("expected non-nil logs list")
	}

	// Should have at least the 2 logs we created
	if len(response.Logs) < 2 {
		t.Errorf("expected at least 2 audit logs, got %d", len(response.Logs))
	}

	// Verify logs have admin username populated
	for _, log := range response.Logs {
		if log.AdminUsername == "" {
			t.Errorf("expected admin username to be populated")
		}
	}

	// Verify target user and metadata are populated for test_action_1
	var matched bool
	for _, log := range response.Logs {
		if log.Action != "test_action_1" {
			continue
		}
		matched = true
		if log.TargetUsername != "audittarget" {
			t.Errorf("expected target username to be populated, got %q", log.TargetUsername)
		}
		if log.Metadata == nil || log.Metadata["note"] != "hello" {
			t.Errorf("expected metadata to include note")
		}
	}
	if !matched {
		t.Errorf("expected to find test_action_1 in response logs")
	}
}

func TestGetAuditLogsWithFilters(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'auditfilteradmin', 'auditfilteradmin@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	otherAdminID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'auditfilteradmin2', 'auditfilteradmin2@example.com', '$2a$12$test', true, now(), now())
	`, otherAdminID)
	if err != nil {
		t.Fatalf("failed to create secondary admin user: %v", err)
	}

	targetID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO users (id, username, email, password_hash, approved_at, created_at)
		VALUES ($1, 'auditfiltertarget', 'auditfiltertarget@example.com', '$2a$12$test', now(), now())
	`, targetID)
	if err != nil {
		t.Fatalf("failed to create target user: %v", err)
	}

	baseTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	_, err = db.Exec(`
		INSERT INTO audit_logs (id, admin_user_id, action, target_user_id, created_at)
		VALUES ($1, $2, 'approve_user', $3, $4)
	`, uuid.New(), adminID, targetID, baseTime)
	if err != nil {
		t.Fatalf("failed to create audit log 1: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO audit_logs (id, admin_user_id, action, created_at)
		VALUES ($1, $2, 'reject_user', $3)
	`, uuid.New(), otherAdminID, baseTime.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("failed to create audit log 2: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO audit_logs (id, admin_user_id, action, related_user_id, created_at)
		VALUES ($1, $2, 'delete_post', $3, $4)
	`, uuid.New(), adminID, targetID, baseTime.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("failed to create audit log 3: %v", err)
	}

	handler := NewAdminHandler(db, nil)

	req := httptest.NewRequest("GET", "/api/v1/admin/audit-logs?action=reject_user", nil)
	w := httptest.NewRecorder()
	handler.GetAuditLogs(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	var actionResponse models.AuditLogsResponse
	if err := json.NewDecoder(w.Body).Decode(&actionResponse); err != nil {
		t.Fatalf("failed to decode action response: %v", err)
	}
	if len(actionResponse.Logs) != 1 || actionResponse.Logs[0].Action != "reject_user" {
		t.Errorf("expected only reject_user log, got %+v", actionResponse.Logs)
	}

	req = httptest.NewRequest("GET", "/api/v1/admin/audit-logs?start=2024-01-02&end=2024-01-02", nil)
	w = httptest.NewRecorder()
	handler.GetAuditLogs(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	var dateResponse models.AuditLogsResponse
	if err := json.NewDecoder(w.Body).Decode(&dateResponse); err != nil {
		t.Fatalf("failed to decode date response: %v", err)
	}
	if len(dateResponse.Logs) != 1 || dateResponse.Logs[0].Action != "reject_user" {
		t.Errorf("expected only reject_user log in date range, got %+v", dateResponse.Logs)
	}

	req = httptest.NewRequest("GET", "/api/v1/admin/audit-logs?admin_user_id="+adminID.String(), nil)
	w = httptest.NewRecorder()
	handler.GetAuditLogs(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	var adminResponse models.AuditLogsResponse
	if err := json.NewDecoder(w.Body).Decode(&adminResponse); err != nil {
		t.Fatalf("failed to decode admin response: %v", err)
	}
	if len(adminResponse.Logs) != 2 {
		t.Errorf("expected 2 logs for admin filter, got %d", len(adminResponse.Logs))
	}

	req = httptest.NewRequest("GET", "/api/v1/admin/audit-logs?target_user_id="+targetID.String(), nil)
	w = httptest.NewRecorder()
	handler.GetAuditLogs(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	var targetResponse models.AuditLogsResponse
	if err := json.NewDecoder(w.Body).Decode(&targetResponse); err != nil {
		t.Fatalf("failed to decode target response: %v", err)
	}
	if len(targetResponse.Logs) != 2 {
		t.Errorf("expected 2 logs for target user filter, got %d", len(targetResponse.Logs))
	}
}

func TestGetAuditLogActions(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	_, err := db.Exec(`
		INSERT INTO audit_logs (id, action, created_at)
		VALUES ($1, 'action_one', now()), ($2, 'action_two', now())
	`, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("failed to create audit logs: %v", err)
	}

	handler := NewAdminHandler(db, nil)
	req := httptest.NewRequest("GET", "/api/v1/admin/audit-logs/actions", nil)
	w := httptest.NewRecorder()

	handler.GetAuditLogActions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response models.AuditLogActionsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(response.Actions) < 2 {
		t.Errorf("expected at least 2 actions, got %d", len(response.Actions))
	}
}

// TestGetAuditLogsMethodNotAllowed tests that POST to GetAuditLogs is rejected
func TestGetAuditLogsMethodNotAllowed(t *testing.T) {
	handler := NewAdminHandler(nil, nil)

	req := httptest.NewRequest("POST", "/api/v1/admin/audit-logs", nil)
	w := httptest.NewRecorder()

	handler.GetAuditLogs(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// TestGetAuditLogsInvalidCursor tests that invalid cursor formats are rejected
func TestGetAuditLogsInvalidCursor(t *testing.T) {
	handler := NewAdminHandler(nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/admin/audit-logs?cursor=not-a-timestamp", nil)
	w := httptest.NewRecorder()

	handler.GetAuditLogs(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestGeneratePasswordResetToken tests generating a password reset token for a user
func TestGeneratePasswordResetToken(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	adminID := uuid.MustParse(testutil.CreateTestUser(t, db, "resetadmin", "resetadmin@example.com", true, true))

	userID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'resetuser', 'reset@example.com', '$2a$12$test', false, now(), now())
	`, userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() { testutil.CleanupRedis(t) })

	handler := NewAdminHandler(db, redisClient)

	reqBody := `{"user_id":"` + userID.String() + `"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/password-reset/generate", strings.NewReader(reqBody))
	req = req.WithContext(createTestUserContext(req.Context(), adminID, "resetadmin", true))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.GeneratePasswordResetToken(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.GeneratePasswordResetTokenResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Token == "" {
		t.Error("expected non-empty token")
	}

	if response.UserID != userID {
		t.Errorf("expected user ID %v, got %v", userID, response.UserID)
	}

	if response.ExpiresAt.IsZero() {
		t.Error("expected non-zero expiration time")
	}

	var auditCount int
	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'generate_password_reset_token' AND related_user_id = $2
	`, adminID, userID).Scan(&auditCount)
	if err != nil {
		t.Fatalf("failed to query audit logs: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("expected 1 audit log entry, got %d", auditCount)
	}
}

// TestGeneratePasswordResetTokenUserNotFound tests generating token for non-existent user
func TestGeneratePasswordResetTokenUserNotFound(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() { testutil.CleanupRedis(t) })

	handler := NewAdminHandler(db, redisClient)

	nonExistentID := uuid.New()
	reqBody := `{"user_id":"` + nonExistentID.String() + `"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/password-reset/generate", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.GeneratePasswordResetToken(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

// TestGeneratePasswordResetTokenUserNotApproved tests generating token for unapproved user
func TestGeneratePasswordResetTokenUserNotApproved(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, created_at)
		VALUES ($1, 'unapproveduser', 'unapproved@example.com', '$2a$12$test', false, now())
	`, userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() { testutil.CleanupRedis(t) })

	handler := NewAdminHandler(db, redisClient)

	reqBody := `{"user_id":"` + userID.String() + `"}`
	req := httptest.NewRequest("POST", "/api/v1/admin/password-reset/generate", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.GeneratePasswordResetToken(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

// TestGeneratePasswordResetTokenMethodNotAllowed tests invalid methods
func TestGeneratePasswordResetTokenMethodNotAllowed(t *testing.T) {
	handler := NewAdminHandler(nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/admin/password-reset/generate", nil)
	w := httptest.NewRecorder()

	handler.GeneratePasswordResetToken(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestAdminTOTPEnrollAndVerify(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	keyBytes := make([]byte, 32)
	for i := range keyBytes {
		keyBytes[i] = byte(i + 1)
	}
	t.Setenv("CLUBHOUSE_TOTP_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(keyBytes))

	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'totpadmin', 'totpadmin@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	handler := NewAdminHandler(db, nil)

	enrollReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/totp/enroll", nil)
	enrollReq = enrollReq.WithContext(createTestUserContext(enrollReq.Context(), adminID, "totpadmin", true))
	enrollRes := httptest.NewRecorder()

	handler.EnrollTOTP(enrollRes, enrollReq)

	if enrollRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, enrollRes.Code, enrollRes.Body.String())
	}

	var enrollBody models.TOTPEnrollResponse
	if err := json.NewDecoder(enrollRes.Body).Decode(&enrollBody); err != nil {
		t.Fatalf("failed to decode enroll response: %v", err)
	}

	if enrollBody.Secret == "" || enrollBody.OtpAuthURL == "" {
		t.Fatalf("expected secret and otpauth url to be set")
	}

	var enrollMetadataBytes []byte
	if err := db.QueryRow(`
		SELECT metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'enroll_mfa'
		ORDER BY created_at DESC
		LIMIT 1
	`, adminID).Scan(&enrollMetadataBytes); err != nil {
		t.Fatalf("failed to query enroll audit log: %v", err)
	}

	var enrollMetadata map[string]interface{}
	if err := json.Unmarshal(enrollMetadataBytes, &enrollMetadata); err != nil {
		t.Fatalf("failed to unmarshal enroll metadata: %v", err)
	}
	if enrollMetadata["method"] != "totp" {
		t.Errorf("expected enroll method 'totp', got %v", enrollMetadata["method"])
	}

	code, err := totp.GenerateCode(enrollBody.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to generate totp code: %v", err)
	}

	verifyPayload, err := json.Marshal(models.TOTPVerifyRequest{Code: code})
	if err != nil {
		t.Fatalf("failed to marshal verify payload: %v", err)
	}

	verifyReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/totp/verify", strings.NewReader(string(verifyPayload)))
	verifyReq = verifyReq.WithContext(createTestUserContext(verifyReq.Context(), adminID, "totpadmin", true))
	verifyRes := httptest.NewRecorder()

	handler.VerifyTOTP(verifyRes, verifyReq)

	if verifyRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, verifyRes.Code, verifyRes.Body.String())
	}

	var enabled bool
	var secret []byte
	if err := db.QueryRow("SELECT totp_enabled, totp_secret_encrypted FROM users WHERE id = $1", adminID).Scan(&enabled, &secret); err != nil {
		t.Fatalf("failed to query totp settings: %v", err)
	}

	if !enabled {
		t.Fatalf("expected totp_enabled to be true")
	}
	if len(secret) == 0 {
		t.Fatalf("expected encrypted secret to be stored")
	}

	var verifyMetadataBytes []byte
	if err := db.QueryRow(`
		SELECT metadata
		FROM audit_logs
		WHERE admin_user_id = $1 AND action = 'enable_mfa'
		ORDER BY created_at DESC
		LIMIT 1
	`, adminID).Scan(&verifyMetadataBytes); err != nil {
		t.Fatalf("failed to query enable audit log: %v", err)
	}

	var verifyMetadata map[string]interface{}
	if err := json.Unmarshal(verifyMetadataBytes, &verifyMetadata); err != nil {
		t.Fatalf("failed to unmarshal enable metadata: %v", err)
	}
	if verifyMetadata["method"] != "totp" {
		t.Errorf("expected enable method 'totp', got %v", verifyMetadata["method"])
	}
}
