// Package testutil provides database and Redis test helpers for integration tests.
package testutil

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

var (
	testDB     *sql.DB
	testDBOnce sync.Once
	testDBErr  error

	miniRedis     *miniredis.Miniredis
	miniRedisOnce sync.Once
)

// GetTestDB returns a shared test database connection.
// It reads the connection string from CLUBHOUSE_TEST_DATABASE_URL env var.
// The database should be a dedicated test database with migrations applied.
func GetTestDB(t *testing.T) *sql.DB {
	t.Helper()

	testDBOnce.Do(func() {
		connStr := os.Getenv("CLUBHOUSE_TEST_DATABASE_URL")
		if connStr == "" {
			testDBErr = fmt.Errorf("CLUBHOUSE_TEST_DATABASE_URL environment variable not set")
			return
		}

		testDB, testDBErr = sql.Open("postgres", connStr)
		if testDBErr != nil {
			testDBErr = fmt.Errorf("failed to open test database: %w", testDBErr)
			return
		}

		if err := testDB.Ping(); err != nil {
			testDBErr = fmt.Errorf("failed to ping test database: %w", err)
			testDB.Close()
			testDB = nil
			return
		}
	})

	if testDBErr != nil {
		t.Fatalf("test database setup failed: %v", testDBErr)
	}

	return testDB
}

// GetTestRedis returns a miniredis instance for testing.
// This provides an in-memory Redis server that doesn't require external dependencies.
func GetTestRedis(t *testing.T) *redis.Client {
	t.Helper()

	miniRedisOnce.Do(func() {
		var err error
		miniRedis, err = miniredis.Run()
		if err != nil {
			t.Fatalf("failed to start miniredis: %v", err)
		}
	})

	if miniRedis == nil {
		t.Fatal("miniredis not initialized")
	}

	client := redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
	})

	return client
}

// CleanupTables truncates all tables for test isolation.
// Call this in a t.Cleanup() or defer to reset state between tests.
func CleanupTables(t *testing.T, db *sql.DB) {
	t.Helper()

	// Use a single TRUNCATE statement with all tables to avoid deadlocks
	// The order matters due to foreign key constraints - CASCADE handles this
	_, err := db.Exec(`
		TRUNCATE TABLE
			admin_config,
			mfa_backup_codes,
			auth_events,
			audit_logs,
			push_subscriptions,
			notifications,
			mentions,
			reactions,
			links,
			comments,
			posts,
			section_subscriptions,
			sections,
			users
		CASCADE
	`)
	if err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}
}

// CleanupRedis flushes all Redis data.
func CleanupRedis(t *testing.T) {
	t.Helper()
	if miniRedis != nil {
		miniRedis.FlushAll()
	}
}

// GetMiniredisServer returns the underlying miniredis server instance.
// This is useful for tests that need access to miniredis-specific functionality
// like FastForward for time manipulation.
func GetMiniredisServer(t *testing.T) *miniredis.Miniredis {
	t.Helper()

	miniRedisOnce.Do(func() {
		var err error
		miniRedis, err = miniredis.Run()
		if err != nil {
			t.Fatalf("failed to start miniredis: %v", err)
		}
	})

	if miniRedis == nil {
		t.Fatal("miniredis not initialized")
	}

	return miniRedis
}

// RunMigrations applies all up migrations to the test database.
// This should be called once during test setup.
func RunMigrations(t *testing.T, db *sql.DB, migrationsDir string) {
	t.Helper()

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	var upMigrations []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			upMigrations = append(upMigrations, entry.Name())
		}
	}

	sort.Strings(upMigrations)

	for _, migration := range upMigrations {
		path := filepath.Join(migrationsDir, migration)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read migration %s: %v", migration, err)
		}

		_, err = db.Exec(string(content))
		if err != nil {
			t.Fatalf("failed to apply migration %s: %v", migration, err)
		}
	}
}

// WithTestTx runs a test function within a transaction and rolls back after.
// This provides test isolation without needing to truncate tables.
func WithTestTx(t *testing.T, db *sql.DB, fn func(tx *sql.Tx)) {
	t.Helper()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			t.Errorf("failed to rollback transaction: %v", err)
		}
	}()

	fn(tx)
}

// RequireTestDB returns a test database connection and fails if not configured.
// Unlike GetTestDB which calls t.Fatalf, this returns the db for more control.
func RequireTestDB(t *testing.T) *sql.DB {
	t.Helper()

	connStr := os.Getenv("CLUBHOUSE_TEST_DATABASE_URL")
	if connStr == "" {
		t.Skip("CLUBHOUSE_TEST_DATABASE_URL not set, skipping database test")
	}

	return GetTestDB(t)
}

// CreateTestUser inserts a test user and returns the user ID.
func CreateTestUser(t *testing.T, db *sql.DB, username, email string, isAdmin, approved bool) string {
	t.Helper()

	var approvedAt interface{} = nil
	if approved {
		approvedAt = "now()"
	}

	var id string
	var query string
	if approved {
		query = `
			INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
			VALUES (gen_random_uuid(), $1, $2, '$2a$12$dummyhash', $3, now(), now())
			RETURNING id
		`
		err := db.QueryRow(query, username, email, isAdmin).Scan(&id)
		if err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}
	} else {
		query = `
			INSERT INTO users (id, username, email, password_hash, is_admin, created_at)
			VALUES (gen_random_uuid(), $1, $2, '$2a$12$dummyhash', $3, now())
			RETURNING id
		`
		err := db.QueryRow(query, username, email, isAdmin).Scan(&id)
		if err != nil {
			t.Fatalf("failed to create test user: %v", err)
		}
	}

	_ = approvedAt

	return id
}

// CreateTestSection inserts a test section and returns the section ID.
func CreateTestSection(t *testing.T, db *sql.DB, name, sectionType string) string {
	t.Helper()

	var id string
	query := `
		INSERT INTO sections (id, name, type, created_at)
		VALUES (gen_random_uuid(), $1, $2, now())
		RETURNING id
	`
	err := db.QueryRow(query, name, sectionType).Scan(&id)
	if err != nil {
		t.Fatalf("failed to create test section: %v", err)
	}

	return id
}

// CreateTestPost inserts a test post and returns the post ID.
func CreateTestPost(t *testing.T, db *sql.DB, userID, sectionID, content string) string {
	t.Helper()

	var id string
	query := `
		INSERT INTO posts (id, user_id, section_id, content, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, now())
		RETURNING id
	`
	err := db.QueryRow(query, userID, sectionID, content).Scan(&id)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	return id
}

// CreateTestComment inserts a test comment and returns the comment ID.
func CreateTestComment(t *testing.T, db *sql.DB, userID, postID, content string) string {
	t.Helper()

	var id string
	query := `
		INSERT INTO comments (id, user_id, post_id, content, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, now())
		RETURNING id
	`
	err := db.QueryRow(query, userID, postID, content).Scan(&id)
	if err != nil {
		t.Fatalf("failed to create test comment: %v", err)
	}

	return id
}
