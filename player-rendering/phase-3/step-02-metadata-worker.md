# Phase 3, Step 2: Metadata Worker

## Overview

Create the worker pool that processes metadata fetch jobs from the Redis queue.

## Detailed Description

Create a new file `backend/internal/services/metadata_worker.go` that provides:

1. **MetadataWorker struct** - Manages a pool of worker goroutines
2. **Start method** - Spawns workers to process jobs
3. **Stop method** - Gracefully shuts down workers
4. **Job processing logic** - Fetches metadata and updates database

### Worker Design

- Configurable number of workers (default: 3)
- Each worker runs in a goroutine, polling the queue
- Graceful shutdown via stop channel
- 5-second blocking timeout on dequeue (allows shutdown checks)
- 30-second timeout per metadata fetch operation

### Dependencies

The worker needs:
- Redis client (for queue operations)
- Database connection (for updating links)
- LinkMetadataService (for fetching metadata)
- A way to publish WebSocket events (added in later step)

## Files to Create

| File | Description |
|------|-------------|
| `backend/internal/services/metadata_worker.go` | Worker pool implementation |
| `backend/internal/services/metadata_worker_test.go` | Unit tests |

## Expected Outcomes

1. Worker pool starts with configured number of workers
2. Workers process jobs from the queue
3. Metadata is fetched and stored in database
4. Workers shut down gracefully on Stop()
5. Failed jobs are acknowledged (removed from processing)
6. All tests pass

## Implementation

```go
package services

import (
    "context"
    "database/sql"
    "encoding/json"
    "sync"
    "time"

    "github.com/google/uuid"
    "github.com/redis/go-redis/v9"
    "github.com/sanderginn/clubhouse/internal/observability"
)

// MetadataWorker manages a pool of workers that process metadata fetch jobs
type MetadataWorker struct {
    redis       *redis.Client
    db          *sql.DB
    linkService *LinkMetadataService
    workerCount int
    stopCh      chan struct{}
    wg          sync.WaitGroup
}

// NewMetadataWorker creates a new metadata worker pool
func NewMetadataWorker(rdb *redis.Client, db *sql.DB, linkService *LinkMetadataService, workerCount int) *MetadataWorker {
    if workerCount <= 0 {
        workerCount = 3
    }
    return &MetadataWorker{
        redis:       rdb,
        db:          db,
        linkService: linkService,
        workerCount: workerCount,
        stopCh:      make(chan struct{}),
    }
}

// Start spawns the worker goroutines
func (w *MetadataWorker) Start(ctx context.Context) {
    observability.LogInfo(ctx, "starting metadata workers", "count", w.workerCount)

    for i := 0; i < w.workerCount; i++ {
        w.wg.Add(1)
        go w.runWorker(ctx, i)
    }
}

// Stop gracefully shuts down all workers
func (w *MetadataWorker) Stop(ctx context.Context) {
    observability.LogInfo(ctx, "stopping metadata workers")
    close(w.stopCh)
    w.wg.Wait()
    observability.LogInfo(ctx, "metadata workers stopped")
}

func (w *MetadataWorker) runWorker(ctx context.Context, workerID int) {
    defer w.wg.Done()

    observability.LogInfo(ctx, "metadata worker started", "worker_id", workerID)

    for {
        select {
        case <-w.stopCh:
            observability.LogInfo(ctx, "metadata worker stopping", "worker_id", workerID)
            return
        case <-ctx.Done():
            observability.LogInfo(ctx, "metadata worker context cancelled", "worker_id", workerID)
            return
        default:
            // Try to get a job with timeout
            job, err := DequeueMetadataJob(ctx, w.redis, 5*time.Second)
            if err != nil {
                observability.LogError(ctx, observability.ErrorLog{
                    Message: "failed to dequeue metadata job",
                    Code:    "METADATA_DEQUEUE_FAILED",
                    Err:     err,
                })
                continue
            }

            if job == nil {
                // Timeout - no job available, continue polling
                continue
            }

            w.processJob(ctx, job, workerID)
        }
    }
}

func (w *MetadataWorker) processJob(ctx context.Context, job *MetadataJob, workerID int) {
    observability.LogDebug(ctx, "processing metadata job",
        "worker_id", workerID,
        "post_id", job.PostID,
        "link_id", job.LinkID,
        "url", job.URL)

    // Create a timeout context for the fetch operation
    fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Fetch metadata using existing link service
    metadata, err := w.linkService.FetchMetadata(fetchCtx, job.URL)
    if err != nil {
        observability.LogError(ctx, observability.ErrorLog{
            Message: "failed to fetch link metadata",
            Code:    "METADATA_FETCH_FAILED",
            Err:     err,
        })
        // Acknowledge job to remove from processing queue even on failure
        AckMetadataJob(ctx, w.redis, *job)
        return
    }

    // Update link in database
    if err := w.updateLinkMetadata(ctx, job.LinkID, metadata); err != nil {
        observability.LogError(ctx, observability.ErrorLog{
            Message: "failed to update link metadata in database",
            Code:    "METADATA_UPDATE_FAILED",
            Err:     err,
        })
        AckMetadataJob(ctx, w.redis, *job)
        return
    }

    observability.LogInfo(ctx, "metadata fetched and stored",
        "worker_id", workerID,
        "post_id", job.PostID,
        "link_id", job.LinkID)

    // Acknowledge successful job
    AckMetadataJob(ctx, w.redis, *job)

    // WebSocket event publishing will be added in a later step
}

func (w *MetadataWorker) updateLinkMetadata(ctx context.Context, linkID uuid.UUID, metadata map[string]interface{}) error {
    metadataJSON, err := json.Marshal(metadata)
    if err != nil {
        return err
    }

    query := `UPDATE links SET metadata = $1, updated_at = NOW() WHERE id = $2`
    _, err = w.db.ExecContext(ctx, query, metadataJSON, linkID)
    return err
}
```

## Test Cases

```go
package services

import (
    "context"
    "database/sql"
    "testing"
    "time"

    "github.com/DATA-DOG/go-sqlmock"
    "github.com/google/uuid"
    "github.com/redis/go-redis/v9"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMetadataWorker_StartStop(t *testing.T) {
    rdb := setupTestRedis(t)
    db, _, _ := sqlmock.New()
    defer db.Close()

    // Create a mock link service (or use a real one with mocked HTTP)
    linkService := &LinkMetadataService{} // Adjust based on actual constructor

    worker := NewMetadataWorker(rdb, db, linkService, 2)

    ctx := context.Background()
    worker.Start(ctx)

    // Give workers time to start
    time.Sleep(100 * time.Millisecond)

    // Stop should complete without hanging
    done := make(chan struct{})
    go func() {
        worker.Stop(ctx)
        close(done)
    }()

    select {
    case <-done:
        // Success
    case <-time.After(10 * time.Second):
        t.Fatal("Stop() timed out")
    }
}

func TestMetadataWorker_ProcessesJobs(t *testing.T) {
    rdb := setupTestRedis(t)
    db, mock, _ := sqlmock.New()
    defer db.Close()

    // Create a mock link service that returns test metadata
    linkService := NewMockLinkMetadataService(map[string]interface{}{
        "title":       "Test Title",
        "description": "Test Description",
    })

    // Expect the database update
    mock.ExpectExec("UPDATE links SET metadata").
        WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
        WillReturnResult(sqlmock.NewResult(0, 1))

    worker := NewMetadataWorker(rdb, db, linkService, 1)

    ctx := context.Background()
    worker.Start(ctx)
    defer worker.Stop(ctx)

    // Enqueue a job
    job := MetadataJob{
        PostID:    uuid.New(),
        LinkID:    uuid.New(),
        URL:       "https://example.com/test",
        CreatedAt: time.Now(),
    }
    err := EnqueueMetadataJob(ctx, rdb, job)
    require.NoError(t, err)

    // Wait for job to be processed
    time.Sleep(1 * time.Second)

    // Verify queue is empty
    queueLen, _ := GetQueueLength(ctx, rdb)
    processingLen, _ := GetProcessingLength(ctx, rdb)
    assert.Equal(t, int64(0), queueLen)
    assert.Equal(t, int64(0), processingLen)

    // Verify database expectations were met
    assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMetadataWorker_HandlesFailedFetch(t *testing.T) {
    rdb := setupTestRedis(t)
    db, _, _ := sqlmock.New()
    defer db.Close()

    // Create a mock link service that returns an error
    linkService := NewMockLinkMetadataServiceWithError()

    worker := NewMetadataWorker(rdb, db, linkService, 1)

    ctx := context.Background()
    worker.Start(ctx)
    defer worker.Stop(ctx)

    // Enqueue a job
    job := MetadataJob{
        PostID:    uuid.New(),
        LinkID:    uuid.New(),
        URL:       "https://example.com/will-fail",
        CreatedAt: time.Now(),
    }
    EnqueueMetadataJob(ctx, rdb, job)

    // Wait for job to be processed
    time.Sleep(1 * time.Second)

    // Job should be acknowledged (removed) even on failure
    queueLen, _ := GetQueueLength(ctx, rdb)
    processingLen, _ := GetProcessingLength(ctx, rdb)
    assert.Equal(t, int64(0), queueLen)
    assert.Equal(t, int64(0), processingLen)
}

func TestMetadataWorker_DefaultWorkerCount(t *testing.T) {
    rdb := setupTestRedis(t)
    db, _, _ := sqlmock.New()
    defer db.Close()

    // Test with invalid worker count
    worker := NewMetadataWorker(rdb, db, nil, 0)
    assert.Equal(t, 3, worker.workerCount)

    worker = NewMetadataWorker(rdb, db, nil, -1)
    assert.Equal(t, 3, worker.workerCount)
}

// Mock link metadata service for testing
type MockLinkMetadataService struct {
    metadata map[string]interface{}
    err      error
}

func NewMockLinkMetadataService(metadata map[string]interface{}) *LinkMetadataService {
    // This needs to be adapted based on actual LinkMetadataService structure
    // You might need to use an interface for better testability
    return &LinkMetadataService{}
}

func NewMockLinkMetadataServiceWithError() *LinkMetadataService {
    return &LinkMetadataService{}
}
```

## Verification

```bash
# Run the tests
cd backend && go test ./internal/services/metadata_worker_test.go -v

# Run all service tests
cd backend && go test ./internal/services/... -v
```

## Notes

- The worker needs access to `LinkMetadataService` - verify its interface/structure
- Consider making `LinkMetadataService.FetchMetadata` an interface for easier testing
- The `updated_at` column may need to be added to the links table if it doesn't exist
- Workers acknowledge jobs even on failure to prevent infinite retries (for MVP)
- Future enhancement: Add retry logic with exponential backoff for transient failures
- Future enhancement: Dead letter queue for permanently failing URLs
- The WebSocket event publishing will be added in step 4
