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

func TestLogWatchCreatesWatchLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchloguser", "watchloguser@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "A great movie")

	service := NewWatchLogService(db, nil)
	watchLog, err := service.LogWatch(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 5, "Amazing film")
	if err != nil {
		t.Fatalf("LogWatch failed: %v", err)
	}
	if watchLog.Rating != 5 {
		t.Fatalf("expected rating 5, got %d", watchLog.Rating)
	}
	if watchLog.Notes == nil || *watchLog.Notes != "Amazing film" {
		t.Fatalf("expected notes %q, got %v", "Amazing film", watchLog.Notes)
	}
	if watchLog.WatchedAt.IsZero() {
		t.Fatalf("expected watched_at to be set")
	}
}

func TestLogWatchAllowsSeriesPosts(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "serieswatchuser", "serieswatchuser@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Series", "series")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "A great show")

	service := NewWatchLogService(db, nil)
	if _, err := service.LogWatch(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 4, ""); err != nil {
		t.Fatalf("LogWatch failed for series post: %v", err)
	}
}

func TestLogWatchRejectsNonMovieOrSeriesPost(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "badwatchpost", "badwatchpost@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "A recipe")

	service := NewWatchLogService(db, nil)
	_, err := service.LogWatch(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 4, "")
	if err == nil {
		t.Fatalf("expected error for non-movie post")
	}
	if err.Error() != "post is not a movie or series" {
		t.Fatalf("expected post type error, got %v", err)
	}
}

func TestLogWatchCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "auditwatchlog", "auditwatchlog@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Movie post")

	service := NewWatchLogService(db, nil)
	_, err := service.LogWatch(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 5, "")
	if err != nil {
		t.Fatalf("LogWatch failed: %v", err)
	}

	var metadataBytes []byte
	query := `
		SELECT metadata
		FROM audit_logs
		WHERE action = 'log_watch' AND target_user_id = $1
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

func TestLogWatchRestoresDeletedWatchLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "restorewatchlog", "restorewatchlog@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Movie post")

	service := NewWatchLogService(db, nil)
	initial, err := service.LogWatch(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 3, "")
	if err != nil {
		t.Fatalf("LogWatch failed: %v", err)
	}
	if err := service.RemoveWatchLog(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID)); err != nil {
		t.Fatalf("RemoveWatchLog failed: %v", err)
	}

	restored, err := service.LogWatch(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 5, "Second watch")
	if err != nil {
		t.Fatalf("LogWatch restore failed: %v", err)
	}
	if restored.ID != initial.ID {
		t.Fatalf("expected restore of same log ID")
	}
	if restored.DeletedAt != nil {
		t.Fatalf("expected deleted_at to be nil")
	}
	if restored.Rating != 5 {
		t.Fatalf("expected rating 5, got %d", restored.Rating)
	}
	if restored.Notes == nil || *restored.Notes != "Second watch" {
		t.Fatalf("expected restored notes %q, got %v", "Second watch", restored.Notes)
	}
}

func TestUpdateWatchLogCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "updatewatchlog", "updatewatchlog@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Movie post")

	service := NewWatchLogService(db, nil)
	_, err := service.LogWatch(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 2, "Old notes")
	if err != nil {
		t.Fatalf("LogWatch failed: %v", err)
	}

	newRating := 4
	newNotes := "Updated notes"
	updated, err := service.UpdateWatchLog(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), &newRating, &newNotes)
	if err != nil {
		t.Fatalf("UpdateWatchLog failed: %v", err)
	}
	if updated.Rating != 4 {
		t.Fatalf("expected rating 4, got %d", updated.Rating)
	}
	if updated.Notes == nil || *updated.Notes != newNotes {
		t.Fatalf("expected notes %q, got %v", newNotes, updated.Notes)
	}

	var metadataBytes []byte
	query := `
		SELECT metadata
		FROM audit_logs
		WHERE action = 'update_watch_log' AND target_user_id = $1
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
	if notesUpdated, ok := metadata["notes_updated"].(bool); !ok || !notesUpdated {
		t.Errorf("expected notes_updated true, got %v", metadata["notes_updated"])
	}
}

func TestRemoveWatchLogCreatesAuditLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "removewatchlog", "removewatchlog@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Movie post")

	service := NewWatchLogService(db, nil)
	_, err := service.LogWatch(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 3, "")
	if err != nil {
		t.Fatalf("LogWatch failed: %v", err)
	}

	if err := service.RemoveWatchLog(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID)); err != nil {
		t.Fatalf("RemoveWatchLog failed: %v", err)
	}

	var deletedAt time.Time
	if err := db.QueryRowContext(context.Background(), `
		SELECT deleted_at FROM watch_logs WHERE user_id = $1 AND post_id = $2
	`, uuid.MustParse(userID), uuid.MustParse(postID)).Scan(&deletedAt); err != nil {
		t.Fatalf("failed to query watch log row: %v", err)
	}
	if deletedAt.IsZero() {
		t.Fatalf("expected deleted_at to be set")
	}

	var metadataBytes []byte
	query := `
		SELECT metadata
		FROM audit_logs
		WHERE action = 'remove_watch_log' AND target_user_id = $1
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

func TestGetPostWatchLogs(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchviewer", "watchviewer@test.com", false, true)
	otherUserID := testutil.CreateTestUser(t, db, "watchother", "watchother@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Movie post")

	service := NewWatchLogService(db, nil)
	_, err := service.LogWatch(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 4, "")
	if err != nil {
		t.Fatalf("LogWatch failed: %v", err)
	}
	_, err = service.LogWatch(context.Background(), uuid.MustParse(otherUserID), uuid.MustParse(postID), 2, "")
	if err != nil {
		t.Fatalf("LogWatch failed: %v", err)
	}

	viewer := uuid.MustParse(userID)
	info, err := service.GetPostWatchLogs(context.Background(), uuid.MustParse(postID), &viewer)
	if err != nil {
		t.Fatalf("GetPostWatchLogs failed: %v", err)
	}

	if info.WatchCount != 2 {
		t.Fatalf("expected watch count 2, got %d", info.WatchCount)
	}
	if info.AvgRating == nil || math.Abs(*info.AvgRating-3.0) > 0.001 {
		t.Fatalf("expected avg rating 3.0, got %v", info.AvgRating)
	}
	if len(info.Logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(info.Logs))
	}
	if !info.ViewerWatched || info.ViewerRating == nil || *info.ViewerRating != 4 {
		t.Fatalf("expected viewer watch data for rating 4")
	}

	for _, entry := range info.Logs {
		if entry.User.ID == uuid.Nil {
			t.Fatalf("expected user id in watch log response")
		}
		if entry.User.Username == "" {
			t.Fatalf("expected username in watch log response")
		}
	}
}

func TestGetUserWatchLogsPagination(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchhistory", "watchhistory@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID1 := testutil.CreateTestPost(t, db, userID, sectionID, "Movie post one")
	postID2 := testutil.CreateTestPost(t, db, userID, sectionID, "Movie post two")

	service := NewWatchLogService(db, nil)
	log1, err := service.LogWatch(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID1), 3, "")
	if err != nil {
		t.Fatalf("LogWatch failed: %v", err)
	}
	log2, err := service.LogWatch(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID2), 4, "")
	if err != nil {
		t.Fatalf("LogWatch failed: %v", err)
	}

	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	if _, err := db.ExecContext(context.Background(), `UPDATE watch_logs SET watched_at = $1 WHERE id = $2`, older, log1.ID); err != nil {
		t.Fatalf("failed to update watched_at: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `UPDATE watch_logs SET watched_at = $1 WHERE id = $2`, newer, log2.ID); err != nil {
		t.Fatalf("failed to update watched_at: %v", err)
	}

	logs, nextCursor, err := service.GetUserWatchLogs(context.Background(), uuid.MustParse(userID), 1, nil)
	if err != nil {
		t.Fatalf("GetUserWatchLogs failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if nextCursor == nil {
		t.Fatalf("expected next cursor")
	}
	if logs[0].Post == nil || logs[0].Post.ID != uuid.MustParse(postID2) {
		t.Fatalf("expected most recent post")
	}

	logs, nextCursor, err = service.GetUserWatchLogs(context.Background(), uuid.MustParse(userID), 1, nextCursor)
	if err != nil {
		t.Fatalf("GetUserWatchLogs with cursor failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if nextCursor != nil {
		t.Fatalf("expected next cursor nil")
	}
	if logs[0].Post == nil || logs[0].Post.ID != uuid.MustParse(postID1) {
		t.Fatalf("expected older post")
	}
}

func TestWatchLogRatingValidation(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchrating", "watchrating@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Movie post")

	service := NewWatchLogService(db, nil)
	if _, err := service.LogWatch(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 6, ""); err == nil {
		t.Fatalf("expected error for invalid rating")
	}

	_, err := service.LogWatch(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), 4, "")
	if err != nil {
		t.Fatalf("LogWatch failed: %v", err)
	}

	invalid := 0
	_, err = service.UpdateWatchLog(context.Background(), uuid.MustParse(userID), uuid.MustParse(postID), &invalid, nil)
	if err == nil {
		t.Fatalf("expected update validation error for invalid rating")
	}
}
