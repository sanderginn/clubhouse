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

func TestLogReadWithAndWithoutRating(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "readloguser", "readloguser@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postIDNoRating := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book without rating"))
	postIDWithRating := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book with rating"))

	service := NewReadLogService(db)

	withoutRating, err := service.LogRead(context.Background(), userID, postIDNoRating, nil)
	if err != nil {
		t.Fatalf("LogRead without rating failed: %v", err)
	}
	if withoutRating.Rating != nil {
		t.Fatalf("expected nil rating, got %v", *withoutRating.Rating)
	}

	rating := 4
	withRating, err := service.LogRead(context.Background(), userID, postIDWithRating, &rating)
	if err != nil {
		t.Fatalf("LogRead with rating failed: %v", err)
	}
	if withRating.Rating == nil || *withRating.Rating != rating {
		t.Fatalf("expected rating %d, got %v", rating, withRating.Rating)
	}
}

func TestRemoveReadLogAndRelog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "readlogrelog", "readlogrelog@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book relog"))

	service := NewReadLogService(db)
	firstRating := 2
	created, err := service.LogRead(context.Background(), userID, postID, &firstRating)
	if err != nil {
		t.Fatalf("LogRead failed: %v", err)
	}

	if err := service.RemoveReadLog(context.Background(), userID, postID); err != nil {
		t.Fatalf("RemoveReadLog failed: %v", err)
	}

	var deletedAt time.Time
	if err := db.QueryRowContext(context.Background(), `
		SELECT deleted_at
		FROM read_logs
		WHERE user_id = $1 AND post_id = $2
	`, userID, postID).Scan(&deletedAt); err != nil {
		t.Fatalf("failed to query deleted read log: %v", err)
	}
	if deletedAt.IsZero() {
		t.Fatalf("expected deleted_at to be set")
	}

	secondRating := 5
	restored, err := service.LogRead(context.Background(), userID, postID, &secondRating)
	if err != nil {
		t.Fatalf("LogRead re-log failed: %v", err)
	}

	if restored.ID != created.ID {
		t.Fatalf("expected restored log id %s, got %s", created.ID, restored.ID)
	}
	if restored.DeletedAt != nil {
		t.Fatalf("expected restored read log to be active")
	}
	if restored.Rating == nil || *restored.Rating != secondRating {
		t.Fatalf("expected rating %d after restore, got %v", secondRating, restored.Rating)
	}
}

func TestUpdateReadRating(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "updatereadrating", "updatereadrating@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book update rating"))

	service := NewReadLogService(db)
	initial := 1
	_, err := service.LogRead(context.Background(), userID, postID, &initial)
	if err != nil {
		t.Fatalf("LogRead failed: %v", err)
	}

	updated, err := service.UpdateRating(context.Background(), userID, postID, 5)
	if err != nil {
		t.Fatalf("UpdateRating failed: %v", err)
	}
	if updated.Rating == nil || *updated.Rating != 5 {
		t.Fatalf("expected updated rating 5, got %v", updated.Rating)
	}
}

func TestGetReadLogAggregations(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	viewerID := uuid.MustParse(testutil.CreateTestUser(t, db, "readviewer", "readviewer@test.com", false, true))
	otherUserID := uuid.MustParse(testutil.CreateTestUser(t, db, "readother", "readother@test.com", false, true))
	thirdUserID := uuid.MustParse(testutil.CreateTestUser(t, db, "readthird", "readthird@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postIDOne := uuid.MustParse(testutil.CreateTestPost(t, db, viewerID.String(), sectionID, "Book one"))
	postIDTwo := uuid.MustParse(testutil.CreateTestPost(t, db, viewerID.String(), sectionID, "Book two"))

	service := NewReadLogService(db)

	ratingFour := 4
	ratingTwo := 2
	ratingFive := 5
	if _, err := service.LogRead(context.Background(), viewerID, postIDOne, &ratingFour); err != nil {
		t.Fatalf("LogRead viewer failed: %v", err)
	}
	if _, err := service.LogRead(context.Background(), otherUserID, postIDOne, nil); err != nil {
		t.Fatalf("LogRead other failed: %v", err)
	}
	if _, err := service.LogRead(context.Background(), thirdUserID, postIDOne, &ratingTwo); err != nil {
		t.Fatalf("LogRead third failed: %v", err)
	}
	if _, err := service.LogRead(context.Background(), otherUserID, postIDTwo, &ratingFive); err != nil {
		t.Fatalf("LogRead second post failed: %v", err)
	}

	postReadLogs, err := service.GetPostReadLogs(context.Background(), postIDOne, &viewerID)
	if err != nil {
		t.Fatalf("GetPostReadLogs failed: %v", err)
	}
	if postReadLogs.ReadCount != 3 {
		t.Fatalf("expected read_count 3, got %d", postReadLogs.ReadCount)
	}
	if postReadLogs.RatedCount != 2 {
		t.Fatalf("expected rated_count 2, got %d", postReadLogs.RatedCount)
	}
	if math.Abs(postReadLogs.AverageRating-3.0) > 0.001 {
		t.Fatalf("expected average_rating 3.0, got %f", postReadLogs.AverageRating)
	}
	if !postReadLogs.ViewerRead {
		t.Fatalf("expected viewer_read true")
	}
	if postReadLogs.ViewerRating == nil || *postReadLogs.ViewerRating != 4 {
		t.Fatalf("expected viewer_rating 4, got %v", postReadLogs.ViewerRating)
	}
	if len(postReadLogs.Readers) != 3 {
		t.Fatalf("expected 3 readers, got %d", len(postReadLogs.Readers))
	}

	logsByPost, err := service.GetReadLogsForPosts(context.Background(), []uuid.UUID{postIDOne, postIDTwo}, &viewerID)
	if err != nil {
		t.Fatalf("GetReadLogsForPosts failed: %v", err)
	}

	if logsByPost[postIDOne].ReadCount != 3 {
		t.Fatalf("expected post one read_count 3, got %d", logsByPost[postIDOne].ReadCount)
	}
	if logsByPost[postIDOne].RatedCount != 2 {
		t.Fatalf("expected post one rated_count 2, got %d", logsByPost[postIDOne].RatedCount)
	}
	if math.Abs(logsByPost[postIDOne].AverageRating-3.0) > 0.001 {
		t.Fatalf("expected post one average_rating 3.0, got %f", logsByPost[postIDOne].AverageRating)
	}
	if !logsByPost[postIDOne].ViewerRead {
		t.Fatalf("expected viewer_read true for post one")
	}
	if logsByPost[postIDOne].ViewerRating == nil || *logsByPost[postIDOne].ViewerRating != 4 {
		t.Fatalf("expected viewer_rating 4 for post one, got %v", logsByPost[postIDOne].ViewerRating)
	}

	if logsByPost[postIDTwo].ReadCount != 1 {
		t.Fatalf("expected post two read_count 1, got %d", logsByPost[postIDTwo].ReadCount)
	}
	if logsByPost[postIDTwo].RatedCount != 1 {
		t.Fatalf("expected post two rated_count 1, got %d", logsByPost[postIDTwo].RatedCount)
	}
	if math.Abs(logsByPost[postIDTwo].AverageRating-5.0) > 0.001 {
		t.Fatalf("expected post two average_rating 5.0, got %f", logsByPost[postIDTwo].AverageRating)
	}
	if logsByPost[postIDTwo].ViewerRead {
		t.Fatalf("expected viewer_read false for post two")
	}
}

func TestGetUserReadHistoryPagination(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "readhistory", "readhistory@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postIDOne := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "History one"))
	postIDTwo := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "History two"))

	service := NewReadLogService(db)
	first, err := service.LogRead(context.Background(), userID, postIDOne, nil)
	if err != nil {
		t.Fatalf("first LogRead failed: %v", err)
	}
	second, err := service.LogRead(context.Background(), userID, postIDTwo, nil)
	if err != nil {
		t.Fatalf("second LogRead failed: %v", err)
	}

	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	if _, err := db.ExecContext(context.Background(), `UPDATE read_logs SET created_at = $1 WHERE id = $2`, older, first.ID); err != nil {
		t.Fatalf("failed to update first created_at: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `UPDATE read_logs SET created_at = $1 WHERE id = $2`, newer, second.ID); err != nil {
		t.Fatalf("failed to update second created_at: %v", err)
	}

	pageOne, nextCursor, err := service.GetUserReadHistory(context.Background(), userID, nil, 1)
	if err != nil {
		t.Fatalf("GetUserReadHistory page one failed: %v", err)
	}
	if len(pageOne) != 1 {
		t.Fatalf("expected 1 result on first page, got %d", len(pageOne))
	}
	if nextCursor == nil {
		t.Fatalf("expected next cursor from first page")
	}
	if pageOne[0].PostID != postIDTwo {
		t.Fatalf("expected newest post id %s, got %s", postIDTwo, pageOne[0].PostID)
	}

	pageTwo, nextCursor, err := service.GetUserReadHistory(context.Background(), userID, nextCursor, 1)
	if err != nil {
		t.Fatalf("GetUserReadHistory page two failed: %v", err)
	}
	if len(pageTwo) != 1 {
		t.Fatalf("expected 1 result on second page, got %d", len(pageTwo))
	}
	if nextCursor != nil {
		t.Fatalf("expected no further cursor after second page")
	}
	if pageTwo[0].PostID != postIDOne {
		t.Fatalf("expected older post id %s, got %s", postIDOne, pageTwo[0].PostID)
	}
}

func TestReadLogAuditEntries(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "readaudit", "readaudit@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Audit book"))

	service := NewReadLogService(db)
	rating := 3
	if _, err := service.LogRead(context.Background(), userID, postID, &rating); err != nil {
		t.Fatalf("LogRead failed: %v", err)
	}
	if _, err := service.UpdateRating(context.Background(), userID, postID, 5); err != nil {
		t.Fatalf("UpdateRating failed: %v", err)
	}
	if err := service.RemoveReadLog(context.Background(), userID, postID); err != nil {
		t.Fatalf("RemoveReadLog failed: %v", err)
	}

	testCases := []struct {
		action          string
		expectPostID    bool
		expectRating    *int
		expectOldRating *int
		expectNewRating *int
	}{
		{action: "log_read", expectPostID: true, expectRating: ptrToInt(3)},
		{action: "update_read_rating", expectPostID: true, expectOldRating: ptrToInt(3), expectNewRating: ptrToInt(5)},
		{action: "remove_read_log", expectPostID: true},
	}

	for _, tc := range testCases {
		var metadataBytes []byte
		if err := db.QueryRowContext(context.Background(), `
			SELECT metadata
			FROM audit_logs
			WHERE action = $1 AND target_user_id = $2
			ORDER BY created_at DESC
			LIMIT 1
		`, tc.action, userID).Scan(&metadataBytes); err != nil {
			t.Fatalf("failed to query audit log %s: %v", tc.action, err)
		}

		metadata := map[string]interface{}{}
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			t.Fatalf("failed to unmarshal metadata for %s: %v", tc.action, err)
		}

		if tc.expectPostID {
			if metadata["post_id"] != postID.String() {
				t.Fatalf("expected post_id %s for %s, got %v", postID, tc.action, metadata["post_id"])
			}
		}
		if tc.expectRating != nil {
			if int(metadata["rating"].(float64)) != *tc.expectRating {
				t.Fatalf("expected rating %d for %s, got %v", *tc.expectRating, tc.action, metadata["rating"])
			}
		}
		if tc.expectOldRating != nil {
			if int(metadata["old_rating"].(float64)) != *tc.expectOldRating {
				t.Fatalf("expected old_rating %d for %s, got %v", *tc.expectOldRating, tc.action, metadata["old_rating"])
			}
		}
		if tc.expectNewRating != nil {
			if int(metadata["new_rating"].(float64)) != *tc.expectNewRating {
				t.Fatalf("expected new_rating %d for %s, got %v", *tc.expectNewRating, tc.action, metadata["new_rating"])
			}
		}
	}
}

func ptrToInt(v int) *int {
	return &v
}
