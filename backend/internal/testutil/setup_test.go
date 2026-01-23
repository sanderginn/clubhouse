package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain can be used by test packages to run migrations once before all tests.
// Example usage in a _test.go file:
//
//	func TestMain(m *testing.M) {
//		testutil.SetupTestDB()
//		os.Exit(m.Run())
//	}
func SetupTestDB() {
	connStr := os.Getenv("CLUBHOUSE_TEST_DATABASE_URL")
	if connStr == "" {
		return
	}

	// Find migrations directory relative to the backend root
	migrationsDir := findMigrationsDir()
	if migrationsDir == "" {
		return
	}

	// Use a temporary testing.T for setup
	t := &testing.T{}
	db := GetTestDB(t)
	if db != nil {
		RunMigrations(t, db, migrationsDir)
	}
}

func findMigrationsDir() string {
	possiblePaths := []string{
		"migrations",
		"../migrations",
		"../../migrations",
		"../../../migrations",
		"backend/migrations",
	}

	cwd, _ := os.Getwd()
	for _, p := range possiblePaths {
		fullPath := filepath.Join(cwd, p)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath
		}
	}

	return ""
}
