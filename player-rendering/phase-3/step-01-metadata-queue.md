# Phase 3, Step 1: Metadata Queue Operations

## Overview

Create the Redis queue infrastructure for async metadata fetching jobs.

## Detailed Description

Create a new file `backend/internal/services/metadata_queue.go` that provides:

1. **Job struct** - Defines the metadata fetch job payload
2. **Enqueue function** - Adds jobs to the Redis queue
3. **Dequeue function** - Retrieves jobs for processing (blocking)
4. **Acknowledge function** - Removes completed jobs from processing queue

### Redis Queue Design

Using a reliable queue pattern with two lists:
- `clubhouse:metadata_queue` - Pending jobs
- `clubhouse:metadata_queue:processing` - Jobs being processed

This allows recovery of in-flight jobs if a worker crashes.

### Queue Operations

| Operation | Redis Command | Description |
|-----------|--------------|-------------|
| Enqueue | `LPUSH` | Add job to head of queue |
| Dequeue | `BRPOPLPUSH` | Atomically move from queue to processing |
| Acknowledge | `LREM` | Remove from processing queue |

### Job Struct

```go
type MetadataJob struct {
    PostID    uuid.UUID `json:"post_id"`
    LinkID    uuid.UUID `json:"link_id"`
    URL       string    `json:"url"`
    CreatedAt time.Time `json:"created_at"`
}
```

## Files to Create

| File | Description |
|------|-------------|
| `backend/internal/services/metadata_queue.go` | Queue operations |
| `backend/internal/services/metadata_queue_test.go` | Unit tests |

## Expected Outcomes

1. Jobs can be enqueued successfully
2. Jobs can be dequeued with blocking wait
3. Jobs can be acknowledged after processing
4. JSON serialization/deserialization works correctly
5. Redis operations are atomic where needed
6. All tests pass

## Implementation

```go
package services

import (
    "context"
    "encoding/json"
    "time"

    "github.com/google/uuid"
    "github.com/redis/go-redis/v9"
)

const (
    // MetadataQueueKey is the Redis key for pending metadata jobs
    MetadataQueueKey = "clubhouse:metadata_queue"
    // MetadataQueueProcessingKey is the Redis key for jobs being processed
    MetadataQueueProcessingKey = "clubhouse:metadata_queue:processing"
)

// MetadataJob represents a link metadata fetch job
type MetadataJob struct {
    PostID    uuid.UUID `json:"post_id"`
    LinkID    uuid.UUID `json:"link_id"`
    URL       string    `json:"url"`
    CreatedAt time.Time `json:"created_at"`
}

// EnqueueMetadataJob adds a link metadata fetch job to the Redis queue
func EnqueueMetadataJob(ctx context.Context, rdb *redis.Client, job MetadataJob) error {
    data, err := json.Marshal(job)
    if err != nil {
        return err
    }
    return rdb.LPush(ctx, MetadataQueueKey, data).Err()
}

// DequeueMetadataJob retrieves the next job from the queue (blocking).
// Returns nil, nil on timeout (no job available).
func DequeueMetadataJob(ctx context.Context, rdb *redis.Client, timeout time.Duration) (*MetadataJob, error) {
    result, err := rdb.BRPopLPush(ctx, MetadataQueueKey, MetadataQueueProcessingKey, timeout).Result()
    if err != nil {
        if err == redis.Nil {
            // Timeout - no job available
            return nil, nil
        }
        return nil, err
    }

    var job MetadataJob
    if err := json.Unmarshal([]byte(result), &job); err != nil {
        // Invalid job data - acknowledge to remove from processing queue
        rdb.LRem(ctx, MetadataQueueProcessingKey, 1, result)
        return nil, err
    }

    return &job, nil
}

// AckMetadataJob removes a completed job from the processing queue
func AckMetadataJob(ctx context.Context, rdb *redis.Client, job MetadataJob) error {
    data, err := json.Marshal(job)
    if err != nil {
        return err
    }
    return rdb.LRem(ctx, MetadataQueueProcessingKey, 1, data).Err()
}

// GetQueueLength returns the number of pending jobs
func GetQueueLength(ctx context.Context, rdb *redis.Client) (int64, error) {
    return rdb.LLen(ctx, MetadataQueueKey).Result()
}

// GetProcessingLength returns the number of jobs being processed
func GetProcessingLength(ctx context.Context, rdb *redis.Client) (int64, error) {
    return rdb.LLen(ctx, MetadataQueueProcessingKey).Result()
}
```

## Test Cases

```go
package services

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/redis/go-redis/v9"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) *redis.Client {
    // Use a test Redis instance or mock
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
        DB:   15, // Use a separate DB for tests
    })

    // Clean up before test
    ctx := context.Background()
    rdb.Del(ctx, MetadataQueueKey, MetadataQueueProcessingKey)

    t.Cleanup(func() {
        rdb.Del(ctx, MetadataQueueKey, MetadataQueueProcessingKey)
        rdb.Close()
    })

    return rdb
}

func TestEnqueueMetadataJob(t *testing.T) {
    rdb := setupTestRedis(t)
    ctx := context.Background()

    job := MetadataJob{
        PostID:    uuid.New(),
        LinkID:    uuid.New(),
        URL:       "https://example.com/test",
        CreatedAt: time.Now(),
    }

    err := EnqueueMetadataJob(ctx, rdb, job)
    require.NoError(t, err)

    // Verify job is in queue
    length, err := GetQueueLength(ctx, rdb)
    require.NoError(t, err)
    assert.Equal(t, int64(1), length)
}

func TestDequeueMetadataJob(t *testing.T) {
    rdb := setupTestRedis(t)
    ctx := context.Background()

    // Enqueue a job
    originalJob := MetadataJob{
        PostID:    uuid.New(),
        LinkID:    uuid.New(),
        URL:       "https://example.com/test",
        CreatedAt: time.Now().Truncate(time.Second), // Truncate for comparison
    }
    err := EnqueueMetadataJob(ctx, rdb, originalJob)
    require.NoError(t, err)

    // Dequeue it
    job, err := DequeueMetadataJob(ctx, rdb, 1*time.Second)
    require.NoError(t, err)
    require.NotNil(t, job)

    assert.Equal(t, originalJob.PostID, job.PostID)
    assert.Equal(t, originalJob.LinkID, job.LinkID)
    assert.Equal(t, originalJob.URL, job.URL)

    // Verify job moved to processing queue
    queueLen, _ := GetQueueLength(ctx, rdb)
    processingLen, _ := GetProcessingLength(ctx, rdb)
    assert.Equal(t, int64(0), queueLen)
    assert.Equal(t, int64(1), processingLen)
}

func TestDequeueMetadataJob_Timeout(t *testing.T) {
    rdb := setupTestRedis(t)
    ctx := context.Background()

    // Dequeue from empty queue with short timeout
    job, err := DequeueMetadataJob(ctx, rdb, 100*time.Millisecond)
    require.NoError(t, err)
    assert.Nil(t, job) // No job available
}

func TestAckMetadataJob(t *testing.T) {
    rdb := setupTestRedis(t)
    ctx := context.Background()

    // Enqueue and dequeue a job
    job := MetadataJob{
        PostID:    uuid.New(),
        LinkID:    uuid.New(),
        URL:       "https://example.com/test",
        CreatedAt: time.Now(),
    }
    EnqueueMetadataJob(ctx, rdb, job)
    dequeuedJob, _ := DequeueMetadataJob(ctx, rdb, 1*time.Second)

    // Acknowledge it
    err := AckMetadataJob(ctx, rdb, *dequeuedJob)
    require.NoError(t, err)

    // Verify processing queue is empty
    processingLen, _ := GetProcessingLength(ctx, rdb)
    assert.Equal(t, int64(0), processingLen)
}

func TestDequeueMetadataJob_FIFO(t *testing.T) {
    rdb := setupTestRedis(t)
    ctx := context.Background()

    // Enqueue jobs in order
    job1 := MetadataJob{PostID: uuid.New(), LinkID: uuid.New(), URL: "https://example.com/1", CreatedAt: time.Now()}
    job2 := MetadataJob{PostID: uuid.New(), LinkID: uuid.New(), URL: "https://example.com/2", CreatedAt: time.Now()}
    job3 := MetadataJob{PostID: uuid.New(), LinkID: uuid.New(), URL: "https://example.com/3", CreatedAt: time.Now()}

    EnqueueMetadataJob(ctx, rdb, job1)
    EnqueueMetadataJob(ctx, rdb, job2)
    EnqueueMetadataJob(ctx, rdb, job3)

    // Dequeue should return in FIFO order
    dequeued1, _ := DequeueMetadataJob(ctx, rdb, 1*time.Second)
    dequeued2, _ := DequeueMetadataJob(ctx, rdb, 1*time.Second)
    dequeued3, _ := DequeueMetadataJob(ctx, rdb, 1*time.Second)

    assert.Equal(t, job1.URL, dequeued1.URL)
    assert.Equal(t, job2.URL, dequeued2.URL)
    assert.Equal(t, job3.URL, dequeued3.URL)
}

func TestEnqueueMetadataJob_MultipleJobs(t *testing.T) {
    rdb := setupTestRedis(t)
    ctx := context.Background()

    // Enqueue multiple jobs
    for i := 0; i < 10; i++ {
        job := MetadataJob{
            PostID:    uuid.New(),
            LinkID:    uuid.New(),
            URL:       "https://example.com/test",
            CreatedAt: time.Now(),
        }
        err := EnqueueMetadataJob(ctx, rdb, job)
        require.NoError(t, err)
    }

    length, _ := GetQueueLength(ctx, rdb)
    assert.Equal(t, int64(10), length)
}
```

## Verification

```bash
# Run the tests (requires Redis running)
cd backend && go test ./internal/services/metadata_queue_test.go -v

# Check queue from Redis CLI
redis-cli LLEN clubhouse:metadata_queue
redis-cli LLEN clubhouse:metadata_queue:processing
```

## Notes

- The `BRPOPLPUSH` command is deprecated in Redis 6.2+ but still supported. The replacement `BLMOVE` can be used if needed.
- Using DB 15 for tests to avoid conflicts with development data
- The queue is FIFO (first in, first out) due to `LPUSH` + `BRPOPLPUSH` from right
- Jobs in the processing queue can be recovered on startup if workers crash
- Consider adding a dead letter queue for repeatedly failing jobs (future enhancement)
