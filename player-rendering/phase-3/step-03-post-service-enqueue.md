# Phase 3, Step 3: Modify Post Service to Enqueue Jobs

## Overview

Remove synchronous metadata fetching from post creation and instead enqueue jobs to the Redis queue.

## Detailed Description

Modify `backend/internal/services/post.go` to:

1. Remove the synchronous `fetchLinkMetadata()` call from `CreatePost()`
2. Store links with `metadata: null` initially
3. Enqueue a metadata job for each link

### Current Behavior (to be removed)

The current `CreatePost()` likely does something like:
```go
// REMOVE THIS - synchronous fetch blocks post creation
for _, link := range req.Links {
    metadata, err := s.linkService.FetchMetadata(ctx, link.URL)
    if err != nil {
        // handle error
    }
    // Store link with metadata
}
```

### New Behavior

```go
// Store links without metadata, enqueue for async fetch
for _, link := range req.Links {
    linkID := uuid.New()

    // Insert link with NULL metadata
    _, err := tx.ExecContext(ctx, `
        INSERT INTO links (id, post_id, url, display_order, metadata, created_at)
        VALUES ($1, $2, $3, $4, NULL, NOW())
    `, linkID, post.ID, link.URL, link.DisplayOrder)

    // Enqueue metadata job (non-blocking, don't fail post creation)
    job := MetadataJob{
        PostID:    post.ID,
        LinkID:    linkID,
        URL:       link.URL,
        CreatedAt: time.Now(),
    }
    if err := EnqueueMetadataJob(ctx, s.redis, job); err != nil {
        observability.LogWarn(ctx, "failed to enqueue metadata job",
            "link_id", linkID, "error", err)
        // Don't fail post creation if queue fails
    }
}
```

### Key Points

1. **Post creation is no longer blocked** by metadata fetching
2. **Links are created immediately** with null metadata
3. **Queue failures don't fail post creation** - they're logged as warnings
4. **Redis client needs to be available** in PostService

## Files to Modify

| File | Changes |
|------|---------|
| `backend/internal/services/post.go` | Remove sync fetch, add job enqueuing |

## Expected Outcomes

1. Post creation returns immediately without waiting for metadata
2. Links are stored with null metadata initially
3. Metadata jobs are enqueued for background processing
4. Queue failures are logged but don't fail post creation
5. Existing tests pass (may need updates for new behavior)
6. New tests verify job enqueuing

## Test Cases

```go
func TestCreatePost_EnqueuesMetadataJobs(t *testing.T) {
    // Setup test dependencies
    rdb := setupTestRedis(t)
    db, mock, _ := sqlmock.New()
    defer db.Close()

    postService := NewPostService(db, rdb, /* other deps */)

    ctx := context.Background()

    // Expect post insert
    mock.ExpectBegin()
    mock.ExpectExec("INSERT INTO posts").
        WillReturnResult(sqlmock.NewResult(1, 1))

    // Expect link insert with NULL metadata
    mock.ExpectExec("INSERT INTO links").
        WithArgs(
            sqlmock.AnyArg(), // link_id
            sqlmock.AnyArg(), // post_id
            "https://bandcamp.com/album/test", // url
            0, // display_order
        ).
        WillReturnResult(sqlmock.NewResult(1, 1))

    mock.ExpectCommit()

    // Create post with link
    post, err := postService.CreatePost(ctx, CreatePostRequest{
        Content: "Check this out",
        Links: []LinkRequest{
            {URL: "https://bandcamp.com/album/test", DisplayOrder: 0},
        },
    })

    require.NoError(t, err)
    require.NotNil(t, post)

    // Verify job was enqueued
    queueLen, _ := GetQueueLength(ctx, rdb)
    assert.Equal(t, int64(1), queueLen)

    // Verify job contents
    job, _ := DequeueMetadataJob(ctx, rdb, 1*time.Second)
    assert.Equal(t, post.ID, job.PostID)
    assert.Equal(t, "https://bandcamp.com/album/test", job.URL)
}

func TestCreatePost_MultipleLinks_EnqueuesAllJobs(t *testing.T) {
    rdb := setupTestRedis(t)
    db, mock, _ := sqlmock.New()
    defer db.Close()

    postService := NewPostService(db, rdb, /* other deps */)
    ctx := context.Background()

    mock.ExpectBegin()
    mock.ExpectExec("INSERT INTO posts").WillReturnResult(sqlmock.NewResult(1, 1))
    mock.ExpectExec("INSERT INTO links").WillReturnResult(sqlmock.NewResult(1, 1))
    mock.ExpectExec("INSERT INTO links").WillReturnResult(sqlmock.NewResult(1, 1))
    mock.ExpectExec("INSERT INTO links").WillReturnResult(sqlmock.NewResult(1, 1))
    mock.ExpectCommit()

    _, err := postService.CreatePost(ctx, CreatePostRequest{
        Content: "Multiple links",
        Links: []LinkRequest{
            {URL: "https://youtube.com/watch?v=abc", DisplayOrder: 0},
            {URL: "https://spotify.com/track/xyz", DisplayOrder: 1},
            {URL: "https://bandcamp.com/album/test", DisplayOrder: 2},
        },
    })

    require.NoError(t, err)

    // All links should have jobs enqueued
    queueLen, _ := GetQueueLength(ctx, rdb)
    assert.Equal(t, int64(3), queueLen)
}

func TestCreatePost_QueueFailure_DoesNotFailPost(t *testing.T) {
    // Use a Redis client that will fail
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:9999", // Non-existent Redis
    })
    defer rdb.Close()

    db, mock, _ := sqlmock.New()
    defer db.Close()

    postService := NewPostService(db, rdb, /* other deps */)
    ctx := context.Background()

    mock.ExpectBegin()
    mock.ExpectExec("INSERT INTO posts").WillReturnResult(sqlmock.NewResult(1, 1))
    mock.ExpectExec("INSERT INTO links").WillReturnResult(sqlmock.NewResult(1, 1))
    mock.ExpectCommit()

    // Post creation should succeed even if queue fails
    post, err := postService.CreatePost(ctx, CreatePostRequest{
        Content: "Test",
        Links: []LinkRequest{
            {URL: "https://example.com", DisplayOrder: 0},
        },
    })

    require.NoError(t, err)
    require.NotNil(t, post)
}

func TestCreatePost_NoLinks_NoJobsEnqueued(t *testing.T) {
    rdb := setupTestRedis(t)
    db, mock, _ := sqlmock.New()
    defer db.Close()

    postService := NewPostService(db, rdb, /* other deps */)
    ctx := context.Background()

    mock.ExpectBegin()
    mock.ExpectExec("INSERT INTO posts").WillReturnResult(sqlmock.NewResult(1, 1))
    mock.ExpectCommit()

    _, err := postService.CreatePost(ctx, CreatePostRequest{
        Content: "No links here",
        Links:   nil,
    })

    require.NoError(t, err)

    // No jobs should be enqueued
    queueLen, _ := GetQueueLength(ctx, rdb)
    assert.Equal(t, int64(0), queueLen)
}
```

## Verification

```bash
# Run post service tests
cd backend && go test ./internal/services/post_test.go -v

# Run all service tests
cd backend && go test ./internal/services/... -v

# Manual test: Create a post, verify it returns immediately
# Check Redis queue: redis-cli LLEN clubhouse:metadata_queue
```

## Implementation Checklist

1. [ ] Find and understand the current `CreatePost` implementation
2. [ ] Identify where metadata fetching happens
3. [ ] Add Redis client to PostService if not present
4. [ ] Remove synchronous metadata fetch call
5. [ ] Update link insert to use NULL metadata
6. [ ] Add job enqueue for each link
7. [ ] Add error handling that logs but doesn't fail
8. [ ] Update/add tests
9. [ ] Verify existing tests still pass

## Notes

- The PostService may need to accept a Redis client in its constructor
- Look for `LinkMetadataService` usage in post.go and remove/comment it for now
- The frontend will handle instant embed rendering (Phase 1-2)
- The metadata will be updated async and sent via WebSocket (Phase 3, Step 4-5)
- Check if there's an `updated_at` trigger on the links table
