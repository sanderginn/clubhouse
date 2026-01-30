package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestAddReactionToPost(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "reactionuser", "reaction@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Test Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test post")

	service := NewReactionService(db)

	_, err := service.AddReactionToPost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), "👍")
	if err != nil {
		t.Fatalf("AddReactionToPost failed: %v", err)
	}

	// Adding same reaction again should not error (upsert)
	_, err = service.AddReactionToPost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), "👍")
	if err != nil {
		t.Fatalf("AddReactionToPost (duplicate) failed: %v", err)
	}
}

func TestAddReactionToPostCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "auditpostreaction", "auditpostreaction@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Audit Post Reaction", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Audit reaction post")

	service := NewReactionService(db)
	_, err := service.AddReactionToPost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), "🔥")
	if err != nil {
		t.Fatalf("AddReactionToPost failed: %v", err)
	}

	var metadataBytes []byte
	query := `
		SELECT metadata
		FROM audit_logs
		WHERE action = 'add_reaction' AND target_user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	if err := db.QueryRowContext(context.Background(), query, uuid.MustParse(userID)).Scan(&metadataBytes); err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if metadata["target"] != "post" {
		t.Errorf("expected target post, got %v", metadata["target"])
	}
	if metadata["target_id"] != postID {
		t.Errorf("expected target_id %s, got %v", postID, metadata["target_id"])
	}
	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
	if metadata["emoji"] != "🔥" {
		t.Errorf("expected emoji 🔥, got %v", metadata["emoji"])
	}
}

func TestRemoveReaction(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "removereactionuser", "removereaction@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Test Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test post")

	service := NewReactionService(db)

	// Add then remove
	_, err := service.AddReactionToPost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), "👍")
	if err != nil {
		t.Fatalf("AddReactionToPost failed: %v", err)
	}

	err = service.RemoveReactionFromPost(context.Background(), uuid.MustParse(postID), "👍", uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("RemoveReactionFromPost failed: %v", err)
	}
}

func TestRemoveReactionFromPostCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "auditremovereaction", "auditremovereaction@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Audit Remove Reaction", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Post for remove reaction")

	service := NewReactionService(db)
	_, err := service.AddReactionToPost(context.Background(), uuid.MustParse(postID), uuid.MustParse(userID), "👍")
	if err != nil {
		t.Fatalf("AddReactionToPost failed: %v", err)
	}

	if err := service.RemoveReactionFromPost(context.Background(), uuid.MustParse(postID), "👍", uuid.MustParse(userID)); err != nil {
		t.Fatalf("RemoveReactionFromPost failed: %v", err)
	}

	var metadataBytes []byte
	query := `
		SELECT metadata
		FROM audit_logs
		WHERE action = 'remove_reaction' AND target_user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	if err := db.QueryRowContext(context.Background(), query, uuid.MustParse(userID)).Scan(&metadataBytes); err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if metadata["target"] != "post" {
		t.Errorf("expected target post, got %v", metadata["target"])
	}
	if metadata["target_id"] != postID {
		t.Errorf("expected target_id %s, got %v", postID, metadata["target_id"])
	}
	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
	if metadata["emoji"] != "👍" {
		t.Errorf("expected emoji 👍, got %v", metadata["emoji"])
	}
}

func TestAddReactionToCommentCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "auditcommentreaction", "auditcommentreaction@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Audit Comment Reaction", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Post for comment reaction")
	commentID := testutil.CreateTestComment(t, db, userID, postID, "Comment for reaction")

	service := NewReactionService(db)
	_, err := service.AddReactionToComment(context.Background(), uuid.MustParse(commentID), uuid.MustParse(userID), "✅")
	if err != nil {
		t.Fatalf("AddReactionToComment failed: %v", err)
	}

	var metadataBytes []byte
	query := `
		SELECT metadata
		FROM audit_logs
		WHERE action = 'add_reaction' AND target_user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	if err := db.QueryRowContext(context.Background(), query, uuid.MustParse(userID)).Scan(&metadataBytes); err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if metadata["target"] != "comment" {
		t.Errorf("expected target comment, got %v", metadata["target"])
	}
	if metadata["target_id"] != commentID {
		t.Errorf("expected target_id %s, got %v", commentID, metadata["target_id"])
	}
	if metadata["comment_id"] != commentID {
		t.Errorf("expected comment_id %s, got %v", commentID, metadata["comment_id"])
	}
	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
	if metadata["emoji"] != "✅" {
		t.Errorf("expected emoji ✅, got %v", metadata["emoji"])
	}
}

func TestRemoveReactionFromCommentCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "auditremovecommentreaction", "auditremovecommentreaction@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Audit Remove Comment Reaction", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Post for remove comment reaction")
	commentID := testutil.CreateTestComment(t, db, userID, postID, "Comment for remove reaction")

	service := NewReactionService(db)
	_, err := service.AddReactionToComment(context.Background(), uuid.MustParse(commentID), uuid.MustParse(userID), "❌")
	if err != nil {
		t.Fatalf("AddReactionToComment failed: %v", err)
	}

	if err := service.RemoveReactionFromComment(context.Background(), uuid.MustParse(commentID), "❌", uuid.MustParse(userID)); err != nil {
		t.Fatalf("RemoveReactionFromComment failed: %v", err)
	}

	var metadataBytes []byte
	query := `
		SELECT metadata
		FROM audit_logs
		WHERE action = 'remove_reaction' AND target_user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	if err := db.QueryRowContext(context.Background(), query, uuid.MustParse(userID)).Scan(&metadataBytes); err != nil {
		t.Fatalf("failed to query audit log: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if metadata["target"] != "comment" {
		t.Errorf("expected target comment, got %v", metadata["target"])
	}
	if metadata["target_id"] != commentID {
		t.Errorf("expected target_id %s, got %v", commentID, metadata["target_id"])
	}
	if metadata["comment_id"] != commentID {
		t.Errorf("expected comment_id %s, got %v", commentID, metadata["comment_id"])
	}
	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
	if metadata["emoji"] != "❌" {
		t.Errorf("expected emoji ❌, got %v", metadata["emoji"])
	}
}

func TestValidateEmoji(t *testing.T) {
	tests := []struct {
		name    string
		emoji   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid emoji",
			emoji:   "👍",
			wantErr: false,
		},
		{
			name:    "valid text emoji",
			emoji:   ":thumbsup:",
			wantErr: false,
		},
		{
			name:    "empty emoji",
			emoji:   "",
			wantErr: true,
			errMsg:  "emoji is required",
		},
		{
			name:    "whitespace only",
			emoji:   "   ",
			wantErr: true,
			errMsg:  "emoji is required",
		},
		{
			name:    "emoji too long",
			emoji:   "12345678901",
			wantErr: true,
			errMsg:  "emoji must be 10 characters or less",
		},
		{
			name:    "emoji at max length",
			emoji:   "1234567890",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEmoji(tt.emoji)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEmoji() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("validateEmoji() error message = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}
