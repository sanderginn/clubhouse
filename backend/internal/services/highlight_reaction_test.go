package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestAddHighlightReactionCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "highlightreactor", "highlightreactor@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Music", "music")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Highlight post")

	linkID := uuid.New()
	highlight := models.Highlight{Timestamp: 30, Label: "Drop"}
	metadata := map[string]interface{}{"highlights": []models.Highlight{highlight}}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("failed to marshal highlight metadata: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO links (id, post_id, url, metadata, created_at)
		VALUES ($1, $2, $3, $4, now())
	`, linkID, uuid.MustParse(postID), "https://example.com", string(metadataBytes))
	if err != nil {
		t.Fatalf("failed to create link: %v", err)
	}

	highlightID, err := models.EncodeHighlightID(linkID, highlight)
	if err != nil {
		t.Fatalf("failed to encode highlight id: %v", err)
	}

	service := NewHighlightReactionService(db)
	response, created, err := service.AddReaction(context.Background(), uuid.MustParse(postID), highlightID, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("AddReaction failed: %v", err)
	}
	if response.HeartCount != 1 {
		t.Fatalf("expected heart count 1, got %d", response.HeartCount)
	}
	if !response.ViewerReacted {
		t.Fatalf("expected viewer reacted true")
	}
	if !created {
		t.Fatalf("expected created to be true")
	}

	_, created, err = service.AddReaction(context.Background(), uuid.MustParse(postID), highlightID, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("AddReaction retry failed: %v", err)
	}
	if created {
		t.Fatalf("expected created false on retry")
	}

	var auditCount int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM audit_logs
		WHERE action = 'add_highlight_reaction' AND target_user_id = $1
	`, uuid.MustParse(userID)).Scan(&auditCount); err != nil {
		t.Fatalf("failed to query audit logs: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 audit log, got %d", auditCount)
	}
}

func TestRemoveHighlightReactionCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "highlightremove", "highlightremove@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Music", "music")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Highlight post")

	linkID := uuid.New()
	highlight := models.Highlight{Timestamp: 45, Label: "Verse"}
	metadata := map[string]interface{}{"highlights": []models.Highlight{highlight}}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("failed to marshal highlight metadata: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO links (id, post_id, url, metadata, created_at)
		VALUES ($1, $2, $3, $4, now())
	`, linkID, uuid.MustParse(postID), "https://example.com", string(metadataBytes))
	if err != nil {
		t.Fatalf("failed to create link: %v", err)
	}

	highlightID, err := models.EncodeHighlightID(linkID, highlight)
	if err != nil {
		t.Fatalf("failed to encode highlight id: %v", err)
	}

	service := NewHighlightReactionService(db)
	_, _, err = service.AddReaction(context.Background(), uuid.MustParse(postID), highlightID, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("AddReaction failed: %v", err)
	}

	response, err := service.RemoveReaction(context.Background(), uuid.MustParse(postID), highlightID, uuid.MustParse(userID))
	if err != nil {
		t.Fatalf("RemoveReaction failed: %v", err)
	}
	if response.HeartCount != 0 {
		t.Fatalf("expected heart count 0, got %d", response.HeartCount)
	}
	if response.ViewerReacted {
		t.Fatalf("expected viewer reacted false")
	}

	var auditCount int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM audit_logs
		WHERE action = 'remove_highlight_reaction' AND target_user_id = $1
	`, uuid.MustParse(userID)).Scan(&auditCount); err != nil {
		t.Fatalf("failed to query audit logs: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 audit log, got %d", auditCount)
	}
}
