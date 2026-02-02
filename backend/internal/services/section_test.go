package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestSectionServiceNilDB(t *testing.T) {
	// Test that NewSectionService with nil db doesn't panic at creation time
	// (actual calls will panic, but that's expected - nil db is programmer error)
	service := NewSectionService(nil)
	if service == nil {
		t.Error("expected non-nil service even with nil db")
	}
}

func TestSectionServiceListSections(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	// Create a test section
	testutil.CreateTestSection(t, db, "Music", "music")

	service := NewSectionService(db)
	sections, err := service.ListSections(context.Background())
	if err != nil {
		t.Fatalf("ListSections failed: %v", err)
	}

	if len(sections) == 0 {
		t.Error("expected at least one section")
	}
}

func TestSectionServiceGetSectionLinksPagination(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "sectionlinksuser", "sectionlinksuser@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Links Section", "general")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Post with links")
	deletedPostID := testutil.CreateTestPost(t, db, userID, sectionID, "Deleted post")

	_, err := db.Exec(`UPDATE posts SET deleted_at = now(), deleted_by_user_id = $1 WHERE id = $2`, userID, deletedPostID)
	if err != nil {
		t.Fatalf("failed to delete post: %v", err)
	}

	now := time.Now().UTC()
	older := now.Add(-2 * time.Hour)
	newer := now.Add(-1 * time.Hour)

	insertTestSectionLink(t, db, postID, "https://example.com/older", map[string]interface{}{"title": "Older"}, older)
	insertTestSectionLink(t, db, postID, "https://example.com/newer", map[string]interface{}{"title": "Newer"}, newer)
	insertTestSectionLink(t, db, deletedPostID, "https://example.com/deleted", nil, now.Add(1*time.Minute))

	service := NewSectionService(db)

	response, err := service.GetSectionLinks(context.Background(), uuid.MustParse(sectionID), nil, 1)
	if err != nil {
		t.Fatalf("GetSectionLinks failed: %v", err)
	}

	if len(response.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(response.Links))
	}

	if response.Links[0].URL != "https://example.com/newer" {
		t.Errorf("expected newest link first, got %s", response.Links[0].URL)
	}

	if response.NextCursor == nil || !response.HasMore {
		t.Fatalf("expected next cursor and hasMore true")
	}

	nextResponse, err := service.GetSectionLinks(context.Background(), uuid.MustParse(sectionID), response.NextCursor, 10)
	if err != nil {
		t.Fatalf("GetSectionLinks with cursor failed: %v", err)
	}

	if len(nextResponse.Links) != 1 {
		t.Fatalf("expected 1 link on second page, got %d", len(nextResponse.Links))
	}

	if nextResponse.Links[0].URL != "https://example.com/older" {
		t.Errorf("expected older link on second page, got %s", nextResponse.Links[0].URL)
	}

	if nextResponse.HasMore || nextResponse.NextCursor != nil {
		t.Errorf("expected no more results after second page")
	}
}

func TestSectionServiceGetSectionLinksInvalidCursor(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	sectionID := testutil.CreateTestSection(t, db, "Cursor Section", "general")

	service := NewSectionService(db)
	_, err := service.GetSectionLinks(context.Background(), uuid.MustParse(sectionID), ptr("not-a-time"), 10)
	if err == nil || err.Error() != "invalid cursor" {
		t.Fatalf("expected invalid cursor error, got %v", err)
	}
}

func TestSectionServiceGetSectionLinksNotFound(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	service := NewSectionService(db)
	_, err := service.GetSectionLinks(context.Background(), uuid.New(), nil, 10)
	if err == nil || err.Error() != "section not found" {
		t.Fatalf("expected section not found error, got %v", err)
	}
}

func insertTestSectionLink(t *testing.T, db *sql.DB, postID, url string, metadata map[string]interface{}, createdAt time.Time) {
	t.Helper()

	var metadataValue interface{}
	if metadata != nil {
		bytes, err := json.Marshal(metadata)
		if err != nil {
			t.Fatalf("failed to marshal metadata: %v", err)
		}
		metadataValue = string(bytes)
	}

	_, err := db.Exec(
		`INSERT INTO links (id, post_id, url, metadata, created_at) VALUES (gen_random_uuid(), $1, $2, $3, $4)`,
		postID, url, metadataValue, createdAt,
	)
	if err != nil {
		t.Fatalf("failed to insert link: %v", err)
	}
}

func ptr(value string) *string {
	return &value
}
