package services

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestPodcastSavesMigrationUpDownRoundTrip(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	upMigrationSQL, err := readMigrationFile("../../migrations/043_create_podcast_saves_table.up.sql")
	if err != nil {
		t.Fatalf("failed to read up migration file: %v", err)
	}

	downMigrationSQL, err := readMigrationFile("../../migrations/043_create_podcast_saves_table.down.sql")
	if err != nil {
		t.Fatalf("failed to read down migration file: %v", err)
	}

	if _, err := db.Exec(downMigrationSQL); err != nil {
		t.Fatalf("failed applying down migration before round-trip: %v", err)
	}

	if _, err := db.Exec(upMigrationSQL); err != nil {
		t.Fatalf("failed applying up migration: %v", err)
	}

	if _, err := db.Exec(downMigrationSQL); err != nil {
		t.Fatalf("failed applying down migration: %v", err)
	}

	if _, err := db.Exec(upMigrationSQL); err != nil {
		t.Fatalf("failed re-applying up migration: %v", err)
	}
}

func TestPodcastSavesActiveUniqueConstraintAndSoftDeleteResave(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "podcastsaveuser", "podcastsave@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Podcasts", "podcast")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Podcast post"))

	ctx := context.Background()

	if _, err := db.ExecContext(ctx,
		`INSERT INTO podcast_saves (user_id, post_id) VALUES ($1, $2)`,
		userID, postID,
	); err != nil {
		t.Fatalf("failed to insert initial podcast save: %v", err)
	}

	if _, err := db.ExecContext(ctx,
		`INSERT INTO podcast_saves (user_id, post_id) VALUES ($1, $2)`,
		userID, postID,
	); err == nil {
		t.Fatalf("expected duplicate active insert to fail")
	}

	if _, err := db.ExecContext(ctx,
		`UPDATE podcast_saves SET deleted_at = now() WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL`,
		userID, postID,
	); err != nil {
		t.Fatalf("failed to soft-delete existing podcast save: %v", err)
	}

	if _, err := db.ExecContext(ctx,
		`INSERT INTO podcast_saves (user_id, post_id) VALUES ($1, $2)`,
		userID, postID,
	); err != nil {
		t.Fatalf("expected re-save after soft delete to succeed, got: %v", err)
	}

	var activeCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM podcast_saves WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL`,
		userID, postID,
	).Scan(&activeCount); err != nil {
		t.Fatalf("failed to count active podcast saves: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected 1 active podcast save, got %d", activeCount)
	}

	var totalCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM podcast_saves WHERE user_id = $1 AND post_id = $2`,
		userID, postID,
	).Scan(&totalCount); err != nil {
		t.Fatalf("failed to count total podcast saves: %v", err)
	}
	if totalCount != 2 {
		t.Fatalf("expected 2 podcast save rows (one soft-deleted, one active), got %d", totalCount)
	}
}

func TestPodcastSavesMigrationCreatesExpectedActiveIndexes(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	rows, err := db.Query(`
		SELECT indexname, indexdef
		FROM pg_indexes
		WHERE schemaname = 'public' AND tablename = 'podcast_saves'
	`)
	if err != nil {
		t.Fatalf("failed to query podcast_saves indexes: %v", err)
	}
	defer rows.Close()

	indexDefs := map[string]string{}
	for rows.Next() {
		var indexName, indexDef string
		if err := rows.Scan(&indexName, &indexDef); err != nil {
			t.Fatalf("failed scanning index row: %v", err)
		}
		indexDefs[indexName] = strings.ToLower(indexDef)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("failed iterating index rows: %v", err)
	}

	assertIndexContains := func(indexName string, snippets ...string) {
		t.Helper()

		def, ok := indexDefs[indexName]
		if !ok {
			t.Fatalf("expected index %q to exist", indexName)
		}
		for _, snippet := range snippets {
			if !strings.Contains(def, snippet) {
				t.Fatalf("expected index %q definition to contain %q, got: %s", indexName, snippet, def)
			}
		}
	}

	assertIndexContains(
		"podcast_saves_user_post_unique",
		"unique index",
		"(user_id, post_id)",
		"deleted_at is null",
	)
	assertIndexContains("idx_podcast_saves_user_id", "(user_id)", "deleted_at is null")
	assertIndexContains("idx_podcast_saves_post_id", "(post_id)", "deleted_at is null")
}
