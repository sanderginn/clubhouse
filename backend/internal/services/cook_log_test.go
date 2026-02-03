package services

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestLogCookCreatesCookLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "cookloguser", "cookloguser@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post")

	service := NewCookLogService(db)
	notes := "Great recipe"
	cookLog, err := service.LogCook(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 4, &notes)
	if err != nil {
		t.Fatalf("LogCook failed: %v", err)
	}
	if cookLog.Rating != 4 {
		t.Fatalf("expected rating 4, got %d", cookLog.Rating)
	}
	if cookLog.Notes == nil || *cookLog.Notes != notes {
		t.Fatalf("expected notes %q, got %v", notes, cookLog.Notes)
	}
}

func TestLogCookCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "auditcooklog", "auditcooklog@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post")

	service := NewCookLogService(db)
	_, err := service.LogCook(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 5, nil)
	if err != nil {
		t.Fatalf("LogCook failed: %v", err)
	}

	var metadataBytes []byte
	query := `
		SELECT metadata
		FROM audit_logs
		WHERE action = 'log_cook' AND target_user_id = $1
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

	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
	if int(metadata["rating"].(float64)) != 5 {
		t.Errorf("expected rating 5, got %v", metadata["rating"])
	}
}

func TestLogCookRestoresDeletedCookLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "restorecooklog", "restorecooklog@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post")

	service := NewCookLogService(db)
	_, err := service.LogCook(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 3, nil)
	if err != nil {
		t.Fatalf("LogCook failed: %v", err)
	}
	if err := service.RemoveCookLog(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID)); err != nil {
		t.Fatalf("RemoveCookLog failed: %v", err)
	}

	updatedNotes := "Second time"
	cookLog, err := service.LogCook(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 5, &updatedNotes)
	if err != nil {
		t.Fatalf("LogCook restore failed: %v", err)
	}
	if cookLog.DeletedAt != nil {
		t.Fatalf("expected deleted_at to be nil")
	}
	if cookLog.Rating != 5 {
		t.Fatalf("expected rating 5, got %d", cookLog.Rating)
	}
	if cookLog.Notes == nil || *cookLog.Notes != updatedNotes {
		t.Fatalf("expected notes %q, got %v", updatedNotes, cookLog.Notes)
	}
}

func TestUpdateCookLogCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "updatecooklog", "updatecooklog@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post")

	service := NewCookLogService(db)
	_, err := service.LogCook(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 2, nil)
	if err != nil {
		t.Fatalf("LogCook failed: %v", err)
	}

	_, err = service.UpdateCookLog(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 4, nil)
	if err != nil {
		t.Fatalf("UpdateCookLog failed: %v", err)
	}

	var metadataBytes []byte
	query := `
		SELECT metadata
		FROM audit_logs
		WHERE action = 'update_cook_log' AND target_user_id = $1
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

	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
	if int(metadata["old_rating"].(float64)) != 2 {
		t.Errorf("expected old_rating 2, got %v", metadata["old_rating"])
	}
	if int(metadata["new_rating"].(float64)) != 4 {
		t.Errorf("expected new_rating 4, got %v", metadata["new_rating"])
	}
}

func TestRemoveCookLogCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "deletecooklog", "deletecooklog@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post")

	service := NewCookLogService(db)
	_, err := service.LogCook(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 3, nil)
	if err != nil {
		t.Fatalf("LogCook failed: %v", err)
	}

	if err := service.RemoveCookLog(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID)); err != nil {
		t.Fatalf("RemoveCookLog failed: %v", err)
	}

	var metadataBytes []byte
	query := `
		SELECT metadata
		FROM audit_logs
		WHERE action = 'delete_cook_log' AND target_user_id = $1
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

	if metadata["post_id"] != postID {
		t.Errorf("expected post_id %s, got %v", postID, metadata["post_id"])
	}
}

func TestGetPostCookLogs(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "cooklogviewer", "cooklogviewer@test.com", false, true)
	otherUserID := testutil.CreateTestUser(t, db, "cooklogother", "cooklogother@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post")

	service := NewCookLogService(db)
	_, err := service.LogCook(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 4, nil)
	if err != nil {
		t.Fatalf("LogCook failed: %v", err)
	}
	_, err = service.LogCook(context.Background(), uuid.MustParse(otherUserID), uuid.MustParse(postID), 2, nil)
	if err != nil {
		t.Fatalf("LogCook failed: %v", err)
	}

	viewer := uuid.MustParse(userID)
	info, err := service.GetPostCookLogs(context.Background(), uuid.MustParse(postID), &viewer)
	if err != nil {
		t.Fatalf("GetPostCookLogs failed: %v", err)
	}

	if info.CookCount != 2 {
		t.Fatalf("expected cook count 2, got %d", info.CookCount)
	}
	if info.AvgRating == nil || math.Abs(*info.AvgRating-3.0) > 0.001 {
		t.Fatalf("expected avg rating 3.0, got %v", info.AvgRating)
	}
	if !info.ViewerCooked || info.ViewerCookLog == nil {
		t.Fatalf("expected viewer cooked data")
	}
	if info.ViewerCookLog.Rating != 4 {
		t.Fatalf("expected viewer rating 4, got %d", info.ViewerCookLog.Rating)
	}
}

func TestGetUserCookLogsPagination(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "cookloghistory", "cookloghistory@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID1 := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post one")
	postID2 := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post two")

	service := NewCookLogService(db)
	log1, err := service.LogCook(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID1), 3, nil)
	if err != nil {
		t.Fatalf("LogCook failed: %v", err)
	}
	log2, err := service.LogCook(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID2), 4, nil)
	if err != nil {
		t.Fatalf("LogCook failed: %v", err)
	}

	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	_, err = db.ExecContext(context.Background(), `UPDATE cook_logs SET created_at = $1 WHERE id = $2`, older, log1.ID)
	if err != nil {
		t.Fatalf("failed to update created_at: %v", err)
	}
	_, err = db.ExecContext(context.Background(), `UPDATE cook_logs SET created_at = $1 WHERE id = $2`, newer, log2.ID)
	if err != nil {
		t.Fatalf("failed to update created_at: %v", err)
	}

	logs, hasMore, nextCursor, err := service.GetUserCookLogs(context.Background(), uuid.MustParse(userID), 1, nil)
	if err != nil {
		t.Fatalf("GetUserCookLogs failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if !hasMore || nextCursor == nil {
		t.Fatalf("expected hasMore true with cursor")
	}
	if logs[0].Post == nil || logs[0].Post.ID != uuid.MustParse(postID2) {
		t.Fatalf("expected most recent post")
	}

	logs, hasMore, nextCursor, err = service.GetUserCookLogs(context.Background(), uuid.MustParse(userID), 1, nextCursor)
	if err != nil {
		t.Fatalf("GetUserCookLogs with cursor failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if hasMore {
		t.Fatalf("expected hasMore false")
	}
	if nextCursor != nil {
		t.Fatalf("expected nextCursor nil")
	}
	if logs[0].Post == nil || logs[0].Post.ID != uuid.MustParse(postID1) {
		t.Fatalf("expected older post")
	}
}

func TestCookLogRatingValidation(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "cooklograting", "cooklograting@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Recipe post")

	service := NewCookLogService(db)
	if _, err := service.LogCook(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 6, nil); err == nil {
		t.Fatalf("expected error for invalid rating")
	}
}
