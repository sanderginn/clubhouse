package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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

func TestSectionServiceListSectionsDeterministicOrderIncludesPodcast(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	testutil.CreateTestSection(t, db, "Recipes", "recipe")
	testutil.CreateTestSection(t, db, "Movies", "movie")
	testutil.CreateTestSection(t, db, "General", "general")
	testutil.CreateTestSection(t, db, "Books", "book")
	testutil.CreateTestSection(t, db, "Series", "series")
	testutil.CreateTestSection(t, db, "Music", "music")
	testutil.CreateTestSection(t, db, "Events", "event")
	testutil.CreateTestSection(t, db, "Podcasts", "podcast")
	testutil.CreateTestSection(t, db, "Zeta Misc", "zeta")
	testutil.CreateTestSection(t, db, "Alpha Misc", "alpha")

	service := NewSectionService(db)
	sections, err := service.ListSections(context.Background())
	if err != nil {
		t.Fatalf("ListSections failed: %v", err)
	}

	gotTypes := make([]string, 0, len(sections))
	for _, section := range sections {
		gotTypes = append(gotTypes, section.Type)
	}

	expectedTypes := []string{
		"general",
		"music",
		"podcast",
		"movie",
		"series",
		"recipe",
		"book",
		"event",
		"alpha",
		"zeta",
	}

	if !reflect.DeepEqual(gotTypes, expectedTypes) {
		t.Fatalf("unexpected section ordering: got %v, want %v", gotTypes, expectedTypes)
	}
}

func TestPodcastSectionMigrationIsIdempotent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	migrationSQL, err := readMigrationFile("../../migrations/042_seed_podcast_section.up.sql")
	if err != nil {
		t.Fatalf("failed to read migration file: %v", err)
	}

	_, err = db.Exec(migrationSQL)
	if err != nil {
		t.Fatalf("failed applying migration (first run): %v", err)
	}

	_, err = db.Exec(migrationSQL)
	if err != nil {
		t.Fatalf("failed applying migration (second run): %v", err)
	}

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM sections WHERE type = 'podcast'`).Scan(&count)
	if err != nil {
		t.Fatalf("failed counting podcast sections: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 podcast section after running migration twice, got %d", count)
	}

	var name string
	err = db.QueryRow(`SELECT name FROM sections WHERE type = 'podcast' LIMIT 1`).Scan(&name)
	if err != nil {
		t.Fatalf("failed reading podcast section: %v", err)
	}
	if name != "Podcasts" {
		t.Fatalf("expected seeded podcast section name to be Podcasts, got %q", name)
	}
}

func TestPodcastSectionDownMigrationIsSafeWithDependentRows(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "podcastsubuser", "podcastsubuser@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Podcasts", "podcast")

	_, err := db.Exec(
		`INSERT INTO section_subscriptions (user_id, section_id, opted_out_at) VALUES ($1, $2, now())`,
		userID,
		sectionID,
	)
	if err != nil {
		t.Fatalf("failed to create dependent section subscription: %v", err)
	}

	downMigrationSQL, err := readMigrationFile("../../migrations/042_seed_podcast_section.down.sql")
	if err != nil {
		t.Fatalf("failed to read down migration file: %v", err)
	}

	_, err = db.Exec(downMigrationSQL)
	if err != nil {
		t.Fatalf("expected down migration to be safe with dependent rows, got error: %v", err)
	}

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM sections WHERE id = $1`, sectionID).Scan(&count)
	if err != nil {
		t.Fatalf("failed verifying podcast section existence after down migration: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected existing podcast section to remain after down migration, got count=%d", count)
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

func readMigrationFile(relativePath string) (string, error) {
	path := filepath.Clean(relativePath)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
