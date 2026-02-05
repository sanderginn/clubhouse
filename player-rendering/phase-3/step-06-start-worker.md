# Phase 3, Step 6: Start Metadata Worker in Server

## Overview

Wire up the metadata worker to start on server startup and stop gracefully on shutdown.

## Detailed Description

Modify `backend/cmd/server/main.go` to:

1. Create the metadata worker with configured worker count
2. Start the worker pool on server startup
3. Stop the worker gracefully on shutdown

### Worker Count Configuration

The number of workers should be configurable via environment variable:
- Environment variable: `METADATA_WORKER_COUNT`
- Default: 3 workers

### Startup Order

1. Initialize database connection
2. Initialize Redis connection
3. Initialize services (including LinkMetadataService)
4. Create and start MetadataWorker
5. Start HTTP server

### Shutdown Order

1. Receive shutdown signal (SIGINT/SIGTERM)
2. Stop accepting new HTTP requests
3. Stop metadata worker (wait for in-flight jobs)
4. Close database connection
5. Close Redis connection

## Files to Modify

| File | Changes |
|------|---------|
| `backend/cmd/server/main.go` | Initialize and manage metadata worker lifecycle |

## Expected Outcomes

1. Metadata worker starts when server starts
2. Worker count is configurable via environment
3. Workers begin processing any queued jobs
4. Server logs worker startup
5. Workers stop gracefully on SIGINT/SIGTERM
6. No goroutine leaks on shutdown

## Implementation

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "strconv"
    "syscall"

    "github.com/sanderginn/clubhouse/internal/observability"
    "github.com/sanderginn/clubhouse/internal/services"
)

func main() {
    ctx := context.Background()

    // ... existing initialization code ...

    // Initialize services
    // db := initDatabase()
    // rdb := initRedis()
    // linkService := services.NewLinkMetadataService(...)

    // Get worker count from environment
    workerCount := getEnvInt("METADATA_WORKER_COUNT", 3)

    // Create metadata worker
    metadataWorker := services.NewMetadataWorker(rdb, db, linkService, workerCount)

    // Start metadata worker
    metadataWorker.Start(ctx)
    observability.LogInfo(ctx, "metadata worker started", "worker_count", workerCount)

    // ... existing server setup ...

    // Setup graceful shutdown
    shutdown := make(chan os.Signal, 1)
    signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

    // Start HTTP server in goroutine
    go func() {
        if err := server.ListenAndServe(); err != nil {
            observability.LogError(ctx, observability.ErrorLog{
                Message: "server error",
                Err:     err,
            })
        }
    }()

    observability.LogInfo(ctx, "server started", "addr", server.Addr)

    // Wait for shutdown signal
    <-shutdown
    observability.LogInfo(ctx, "shutdown signal received")

    // Create shutdown context with timeout
    shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Stop HTTP server
    if err := server.Shutdown(shutdownCtx); err != nil {
        observability.LogError(ctx, observability.ErrorLog{
            Message: "server shutdown error",
            Err:     err,
        })
    }

    // Stop metadata worker
    metadataWorker.Stop(ctx)

    // Close database
    if err := db.Close(); err != nil {
        observability.LogError(ctx, observability.ErrorLog{
            Message: "database close error",
            Err:     err,
        })
    }

    // Close Redis
    if err := rdb.Close(); err != nil {
        observability.LogError(ctx, observability.ErrorLog{
            Message: "redis close error",
            Err:     err,
        })
    }

    observability.LogInfo(ctx, "server stopped")
}

func getEnvInt(key string, defaultVal int) int {
    if val := os.Getenv(key); val != "" {
        if i, err := strconv.Atoi(val); err == nil {
            return i
        }
    }
    return defaultVal
}
```

## Test Cases

Since main.go integration testing is complex, focus on:

### Manual Testing Checklist

1. **Server Startup**
   ```bash
   # Start server
   task dev:up

   # Check logs for worker startup
   task dev:logs-backend | grep "metadata worker"
   # Should see: "metadata worker started" with worker_count
   ```

2. **Worker Count Configuration**
   ```bash
   # Set custom worker count
   export METADATA_WORKER_COUNT=5
   task dev:up

   # Verify in logs
   task dev:logs-backend | grep "worker_count"
   ```

3. **Graceful Shutdown**
   ```bash
   # Start server
   task dev:up

   # In another terminal, enqueue some jobs by creating posts with links
   # Then stop the server
   task dev:down

   # Check logs for graceful shutdown
   task dev:logs-backend | grep -E "(stopping|stopped)"
   # Should see: "metadata worker stopping", "metadata worker stopped"
   ```

4. **Job Processing on Startup**
   ```bash
   # Manually enqueue a job
   redis-cli LPUSH clubhouse:metadata_queue '{"post_id":"...", "link_id":"...", "url":"https://bandcamp.com/test", "created_at":"..."}'

   # Start server
   task dev:up

   # Job should be processed (check queue is empty)
   redis-cli LLEN clubhouse:metadata_queue
   # Should be 0
   ```

### Unit Tests (for helper function)

```go
func TestGetEnvInt(t *testing.T) {
    tests := []struct {
        name       string
        envValue   string
        defaultVal int
        expected   int
    }{
        {"uses default when not set", "", 3, 3},
        {"uses env value when set", "5", 3, 5},
        {"uses default on invalid value", "abc", 3, 3},
        {"uses default on empty string", "", 10, 10},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if tt.envValue != "" {
                os.Setenv("TEST_ENV_INT", tt.envValue)
                defer os.Unsetenv("TEST_ENV_INT")
            }
            result := getEnvInt("TEST_ENV_INT", tt.defaultVal)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

## Verification

```bash
# Build and run
cd backend && go build -o server ./cmd/server && ./server

# Or use task
task dev:up
task dev:logs-backend

# Verify workers are running by creating a post with link
# and checking queue gets processed

# Test shutdown
# Press Ctrl+C or send SIGTERM
# Verify clean shutdown in logs
```

## Docker Compose Changes

If using docker-compose, ensure the environment variable can be set:

```yaml
services:
  backend:
    environment:
      - METADATA_WORKER_COUNT=${METADATA_WORKER_COUNT:-3}
```

## Notes

- Check existing main.go structure and follow its patterns
- The server might already have graceful shutdown handling - integrate with it
- Ensure LinkMetadataService is properly initialized before passing to worker
- The worker should handle the case where Redis has queued jobs from previous runs
- Consider adding a startup log that shows queue depth (jobs waiting to be processed)
- In production, monitor worker health via metrics (queue depth, processing rate)
