package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestRegisterUserCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	service := NewUserService(db)
	req := &models.RegisterRequest{
		Username: "auditreg",
		Email:    "auditreg@example.com",
		Password: "LongPassword1234",
	}

	user, err := service.RegisterUser(context.Background(), req)
	if err != nil {
		t.Fatalf("RegisterUser failed: %v", err)
	}

	var adminUserID uuid.NullUUID
	var targetUserID uuid.UUID
	var metadataBytes []byte
	query := `
		SELECT admin_user_id, target_user_id, metadata
		FROM audit_logs
		WHERE action = 'register_user' AND target_user_id = $1
	`
	if err := db.QueryRowContext(context.Background(), query, user.ID).Scan(&adminUserID, &targetUserID, &metadataBytes); err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	if adminUserID.Valid {
		t.Errorf("expected admin_user_id to be NULL for registration audit log")
	}
	if targetUserID != user.ID {
		t.Errorf("expected target_user_id %s, got %s", user.ID, targetUserID)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if metadata["username"] != req.Username {
		t.Errorf("expected metadata username %q, got %v", req.Username, metadata["username"])
	}
	if metadata["email"] != req.Email {
		t.Errorf("expected metadata email %q, got %v", req.Email, metadata["email"])
	}
}

func TestUpdateProfileCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, bio, profile_picture_url, is_admin, approved_at, created_at)
		VALUES ($1, 'auditprofile', 'auditprofile@example.com', '$2a$12$test', 'old bio', 'https://old.example.com/avatar.png', false, now(), now())
	`, userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	service := NewUserService(db)
	newBio := "new bio"
	newProfile := "https://new.example.com/avatar.png"
	req := &models.UpdateUserRequest{
		Bio:               &newBio,
		ProfilePictureUrl: &newProfile,
	}

	if _, err := service.UpdateProfile(context.Background(), userID, req); err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}

	var adminUserID uuid.NullUUID
	var targetUserID uuid.UUID
	var metadataBytes []byte
	query := `
		SELECT admin_user_id, target_user_id, metadata
		FROM audit_logs
		WHERE action = 'update_profile' AND target_user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	if err := db.QueryRowContext(context.Background(), query, userID).Scan(&adminUserID, &targetUserID, &metadataBytes); err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	if adminUserID.Valid {
		t.Errorf("expected admin_user_id to be NULL for profile update audit log")
	}
	if targetUserID != userID {
		t.Errorf("expected target_user_id %s, got %s", userID, targetUserID)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	changedFields, ok := metadata["changed_fields"].([]interface{})
	if !ok {
		t.Fatalf("expected changed_fields list in metadata")
	}
	if len(changedFields) != 2 {
		t.Errorf("expected 2 changed fields, got %d", len(changedFields))
	}

	changes, ok := metadata["changes"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected changes map in metadata")
	}

	bioChange, ok := changes["bio"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected bio change metadata")
	}
	if bioChange["old"] != "old bio" {
		t.Errorf("expected bio old value 'old bio', got %v", bioChange["old"])
	}
	if bioChange["new"] != newBio {
		t.Errorf("expected bio new value %q, got %v", newBio, bioChange["new"])
	}

	profileChange, ok := changes["profile_picture_url"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected profile_picture_url change metadata")
	}
	if profileChange["old"] != "https://old.example.com/avatar.png" {
		t.Errorf("expected profile old value 'https://old.example.com/avatar.png', got %v", profileChange["old"])
	}
	if profileChange["new"] != newProfile {
		t.Errorf("expected profile new value %q, got %v", newProfile, profileChange["new"])
	}
}
