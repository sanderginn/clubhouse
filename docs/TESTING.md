# Testing Guide

This document explains how to run tests for the Clubhouse backend.

## Quick Start

### Running Tests Without Database

```bash
cd backend
go test ./...
```

This runs all tests, but DB-backed tests will be skipped.

### Running Tests With Database

1. **Start the test database containers:**

```bash
docker compose -f docker-compose.test.yml up -d
```

2. **Wait for containers to be healthy** (usually a few seconds)

3. **Run migrations on the test database:**

```bash
export CLUBHOUSE_TEST_DATABASE_URL="postgres://clubhouse_test:clubhouse_test@localhost:5433/clubhouse_test?sslmode=disable"
cd backend && cat migrations/*.up.sql | psql "$CLUBHOUSE_TEST_DATABASE_URL"
```

4. **Run tests:**

```bash
export CLUBHOUSE_TEST_DATABASE_URL="postgres://clubhouse_test:clubhouse_test@localhost:5433/clubhouse_test?sslmode=disable"
cd backend && go test -v ./...
```

5. **Cleanup:**

```bash
docker compose -f docker-compose.test.yml down
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CLUBHOUSE_TEST_DATABASE_URL` | PostgreSQL connection string for test database | (none - tests skip if not set) |

## Test Database Configuration

The test database uses:
- **Port**: 5433 (to avoid conflict with development database on 5432)
- **User**: `clubhouse_test`
- **Password**: `clubhouse_test`
- **Database**: `clubhouse_test`

Data is stored in tmpfs for faster tests and automatic cleanup on container stop.

## Redis for Tests

Tests use [miniredis](https://github.com/alicebob/miniredis) - an in-memory Redis server for Go tests. No external Redis container is required for running tests.

## Test Utilities

The `internal/testutil` package provides helpers for database tests:

```go
import "github.com/sanderginn/clubhouse/internal/testutil"

func TestExample(t *testing.T) {
    db := testutil.RequireTestDB(t)
    t.Cleanup(func() { testutil.CleanupTables(t, db) })

    // Create test data
    userID := testutil.CreateTestUser(t, db, "testuser", "test@example.com", false, true)
    sectionID := testutil.CreateTestSection(t, db, "Music", "music")
    postID := testutil.CreateTestPost(t, db, userID, sectionID, "Hello world")

    // Your test logic here
}
```

### Available Helpers

- `RequireTestDB(t)` - Returns a test database connection (skips test if not configured)
- `GetTestRedis(t)` - Returns a miniredis client
- `CleanupTables(t, db)` - Truncates all tables (call in t.Cleanup)
- `CleanupRedis(t)` - Flushes all Redis data
- `CreateTestUser(t, db, username, email, isAdmin, approved)` - Creates a user, returns ID
- `CreateTestSection(t, db, name, sectionType)` - Creates a section, returns ID
- `CreateTestPost(t, db, userID, sectionID, content)` - Creates a post, returns ID
- `CreateTestComment(t, db, userID, postID, content)` - Creates a comment, returns ID

## Running Specific Tests

```bash
# Run a single test file
go test ./internal/handlers/admin_test.go -v

# Run tests matching a pattern
go test ./internal/handlers/... -run TestApprove -v

# Run tests in a specific package
go test ./internal/services/... -v
```

## CI Configuration

In CI (Buildkite), set `CLUBHOUSE_TEST_DATABASE_URL` in the pipeline environment to enable DB-backed tests. The pipeline should:

1. Start a PostgreSQL service container
2. Run migrations
3. Set the environment variable
4. Run tests

Example Buildkite step:
```yaml
- label: "Backend Tests"
  command: ".buildkite/scripts/backend-test.sh"
  env:
    CLUBHOUSE_TEST_DATABASE_URL: "postgres://clubhouse_test:clubhouse_test@localhost:5433/clubhouse_test?sslmode=disable"
  plugins:
    - docker-compose#v4.0.0:
        run: postgres-test
```

## Troubleshooting

### Tests skip with "CLUBHOUSE_TEST_DATABASE_URL not set"

Set the environment variable as shown above, or start the test containers.

### "relation does not exist" errors

Run migrations on the test database:
```bash
cat migrations/*.up.sql | psql "$CLUBHOUSE_TEST_DATABASE_URL"
```

### Tests fail with connection errors

Ensure the test database container is running:
```bash
docker compose -f docker-compose.test.yml ps
```

### Cleanup between test runs

If tests leave stale data, you can reset the database:
```bash
docker compose -f docker-compose.test.yml down -v
docker compose -f docker-compose.test.yml up -d
# Re-run migrations
```
