package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestPodcastSaveAndUnsaveIdempotentWithRestoreAndAudit(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "podcastsaveidempotent", "podcastsaveidempotent@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Podcasts", "podcast")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Podcast post"))

	service := NewPodcastSaveService(db)

	firstSave, err := service.SavePodcast(context.Background(), userID, postID)
	if err != nil {
		t.Fatalf("first SavePodcast failed: %v", err)
	}

	secondSave, err := service.SavePodcast(context.Background(), userID, postID)
	if err != nil {
		t.Fatalf("second SavePodcast failed: %v", err)
	}
	if secondSave.ID != firstSave.ID {
		t.Fatalf("expected duplicate save to keep same save row, got %s and %s", firstSave.ID, secondSave.ID)
	}

	assertPodcastSaveCounts(t, db, userID, postID, 1, 1)
	assertPodcastAuditCount(t, db, "save_podcast", userID, 1)

	if err := service.UnsavePodcast(context.Background(), userID, postID); err != nil {
		t.Fatalf("first UnsavePodcast failed: %v", err)
	}
	if err := service.UnsavePodcast(context.Background(), userID, postID); err != nil {
		t.Fatalf("second UnsavePodcast should be idempotent, got: %v", err)
	}

	assertPodcastSaveCounts(t, db, userID, postID, 0, 1)
	assertPodcastAuditCount(t, db, "unsave_podcast", userID, 1)

	thirdSave, err := service.SavePodcast(context.Background(), userID, postID)
	if err != nil {
		t.Fatalf("third SavePodcast failed: %v", err)
	}
	if thirdSave.ID != firstSave.ID {
		t.Fatalf("expected resave to restore same row %s, got %s", firstSave.ID, thirdSave.ID)
	}

	assertPodcastSaveCounts(t, db, userID, postID, 1, 1)
	assertPodcastAuditCount(t, db, "save_podcast", userID, 2)

	saveMetadata := mustQueryPodcastAuditMetadata(t, db, "save_podcast", userID)
	if saveMetadata["post_id"] != postID.String() {
		t.Fatalf("expected save_podcast metadata post_id %s, got %v", postID.String(), saveMetadata["post_id"])
	}
}

func TestPodcastSaveRejectsNonPodcastPost(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "podcastsaveinvalid", "podcastsaveinvalid@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Recipe post"))

	service := NewPodcastSaveService(db)

	_, err := service.SavePodcast(context.Background(), userID, postID)
	if err == nil {
		t.Fatal("expected SavePodcast to fail for non-podcast post")
	}
	if !strings.Contains(err.Error(), "podcast post not found") {
		t.Fatalf("expected podcast post guard error, got %v", err)
	}

	err = service.UnsavePodcast(context.Background(), userID, postID)
	if err == nil {
		t.Fatal("expected UnsavePodcast to fail for non-podcast post")
	}
	if !strings.Contains(err.Error(), "podcast post not found") {
		t.Fatalf("expected podcast post guard error, got %v", err)
	}
}

func TestGetPostPodcastSaveInfoIncludesViewerAndAggregate(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userA := uuid.MustParse(testutil.CreateTestUser(t, db, "podcastinfousera", "podcastinfousera@test.com", false, true))
	userB := uuid.MustParse(testutil.CreateTestUser(t, db, "podcastinfouserb", "podcastinfouserb@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Podcasts", "podcast")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userA.String(), sectionID, "Podcast post"))

	service := NewPodcastSaveService(db)
	if _, err := service.SavePodcast(context.Background(), userA, postID); err != nil {
		t.Fatalf("SavePodcast userA failed: %v", err)
	}
	if _, err := service.SavePodcast(context.Background(), userB, postID); err != nil {
		t.Fatalf("SavePodcast userB failed: %v", err)
	}

	info, err := service.GetPostPodcastSaveInfo(context.Background(), postID, &userA)
	if err != nil {
		t.Fatalf("GetPostPodcastSaveInfo failed: %v", err)
	}

	if info.SaveCount != 2 {
		t.Fatalf("expected save count 2, got %d", info.SaveCount)
	}
	if len(info.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(info.Users))
	}
	if !info.ViewerSaved {
		t.Fatal("expected viewer_saved true")
	}
}

func TestListSectionSavedPodcastPostsPaginatesAndGuardsSectionType(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "podcastlistuser", "podcastlistuser@test.com", false, true))
	podcastSectionID := uuid.MustParse(testutil.CreateTestSection(t, db, "Podcasts", "podcast"))
	otherPodcastSectionID := uuid.MustParse(testutil.CreateTestSection(t, db, "Other Podcasts", "podcast"))
	recipeSectionID := uuid.MustParse(testutil.CreateTestSection(t, db, "Recipes", "recipe"))

	postA := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), podcastSectionID.String(), "Podcast A"))
	postB := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), podcastSectionID.String(), "Podcast B"))
	postC := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), podcastSectionID.String(), "Podcast C"))
	postOtherSection := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), otherPodcastSectionID.String(), "Other section podcast"))

	service := NewPodcastSaveService(db)
	if _, err := service.SavePodcast(context.Background(), userID, postA); err != nil {
		t.Fatalf("SavePodcast postA failed: %v", err)
	}
	if _, err := service.SavePodcast(context.Background(), userID, postB); err != nil {
		t.Fatalf("SavePodcast postB failed: %v", err)
	}
	if _, err := service.SavePodcast(context.Background(), userID, postC); err != nil {
		t.Fatalf("SavePodcast postC failed: %v", err)
	}
	if _, err := service.SavePodcast(context.Background(), userID, postOtherSection); err != nil {
		t.Fatalf("SavePodcast postOtherSection failed: %v", err)
	}

	oldest := time.Now().UTC().Add(-3 * time.Hour)
	middle := oldest.Add(1 * time.Hour)
	newest := middle.Add(1 * time.Hour)
	if _, err := db.ExecContext(context.Background(), `
		UPDATE podcast_saves SET created_at = $1 WHERE user_id = $2 AND post_id = $3
	`, oldest, userID, postA); err != nil {
		t.Fatalf("failed to set created_at for postA: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `
		UPDATE podcast_saves SET created_at = $1 WHERE user_id = $2 AND post_id = $3
	`, middle, userID, postB); err != nil {
		t.Fatalf("failed to set created_at for postB: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `
		UPDATE podcast_saves SET created_at = $1 WHERE user_id = $2 AND post_id = $3
	`, newest, userID, postC); err != nil {
		t.Fatalf("failed to set created_at for postC: %v", err)
	}

	pageOne, err := service.ListSectionSavedPodcastPosts(context.Background(), podcastSectionID, userID, nil, 2)
	if err != nil {
		t.Fatalf("ListSectionSavedPodcastPosts page 1 failed: %v", err)
	}
	if len(pageOne.Posts) != 2 {
		t.Fatalf("expected 2 posts in page 1, got %d", len(pageOne.Posts))
	}
	if !pageOne.HasMore {
		t.Fatal("expected page 1 has_more true")
	}
	if pageOne.NextCursor == nil {
		t.Fatal("expected non-nil next cursor on page 1")
	}
	if pageOne.Posts[0].ID != postC {
		t.Fatalf("expected latest post first (%s), got %s", postC, pageOne.Posts[0].ID)
	}
	if pageOne.Posts[1].ID != postB {
		t.Fatalf("expected second latest post second (%s), got %s", postB, pageOne.Posts[1].ID)
	}

	pageTwo, err := service.ListSectionSavedPodcastPosts(context.Background(), podcastSectionID, userID, pageOne.NextCursor, 2)
	if err != nil {
		t.Fatalf("ListSectionSavedPodcastPosts page 2 failed: %v", err)
	}
	if len(pageTwo.Posts) != 1 {
		t.Fatalf("expected 1 post in page 2, got %d", len(pageTwo.Posts))
	}
	if pageTwo.HasMore {
		t.Fatal("expected page 2 has_more false")
	}
	if pageTwo.NextCursor != nil {
		t.Fatalf("expected nil next cursor on final page, got %v", *pageTwo.NextCursor)
	}
	if pageTwo.Posts[0].ID != postA {
		t.Fatalf("expected oldest post on page 2 (%s), got %s", postA, pageTwo.Posts[0].ID)
	}

	_, err = service.ListSectionSavedPodcastPosts(context.Background(), recipeSectionID, userID, nil, 2)
	if err == nil {
		t.Fatal("expected non-podcast section listing to fail")
	}
	if !strings.Contains(err.Error(), "section is not podcast") {
		t.Fatalf("expected non-podcast section error, got %v", err)
	}
}

func assertPodcastSaveCounts(t *testing.T, db *sql.DB, userID, postID uuid.UUID, expectedActive, expectedTotal int) {
	t.Helper()

	var activeCount int
	if err := db.QueryRowContext(context.Background(), `
		SELECT COUNT(*)
		FROM podcast_saves
		WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL
	`, userID, postID).Scan(&activeCount); err != nil {
		t.Fatalf("failed to query active podcast save count: %v", err)
	}
	if activeCount != expectedActive {
		t.Fatalf("expected %d active podcast saves, got %d", expectedActive, activeCount)
	}

	var totalCount int
	if err := db.QueryRowContext(context.Background(), `
		SELECT COUNT(*)
		FROM podcast_saves
		WHERE user_id = $1 AND post_id = $2
	`, userID, postID).Scan(&totalCount); err != nil {
		t.Fatalf("failed to query total podcast save count: %v", err)
	}
	if totalCount != expectedTotal {
		t.Fatalf("expected %d total podcast save rows, got %d", expectedTotal, totalCount)
	}
}

func assertPodcastAuditCount(t *testing.T, db *sql.DB, action string, userID uuid.UUID, expected int) {
	t.Helper()

	var count int
	if err := db.QueryRowContext(context.Background(), `
		SELECT COUNT(*)
		FROM audit_logs
		WHERE action = $1 AND target_user_id = $2
	`, action, userID).Scan(&count); err != nil {
		t.Fatalf("failed to query %s audit count: %v", action, err)
	}
	if count != expected {
		t.Fatalf("expected %d %s audit rows, got %d", expected, action, count)
	}
}

func mustQueryPodcastAuditMetadata(t *testing.T, db *sql.DB, action string, userID uuid.UUID) map[string]interface{} {
	t.Helper()

	var metadataRaw []byte
	if err := db.QueryRowContext(context.Background(), `
		SELECT metadata
		FROM audit_logs
		WHERE action = $1 AND target_user_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, action, userID).Scan(&metadataRaw); err != nil {
		t.Fatalf("failed to query %s audit metadata: %v", action, err)
	}

	metadata := map[string]interface{}{}
	if err := json.Unmarshal(metadataRaw, &metadata); err != nil {
		t.Fatalf("failed to unmarshal %s audit metadata: %v", action, err)
	}
	return metadata
}
