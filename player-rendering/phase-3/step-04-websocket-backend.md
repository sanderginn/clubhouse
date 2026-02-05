# Phase 3, Step 4: WebSocket Event for Metadata Updates (Backend)

## Overview

Add the ability to publish WebSocket events when link metadata is fetched, so frontends can update in real-time.

## Detailed Description

Modify the backend to publish a `link_metadata_updated` event via Redis pub/sub when metadata fetching completes.

### Event Structure

```json
{
  "type": "link_metadata_updated",
  "data": {
    "post_id": "uuid",
    "link_id": "uuid",
    "metadata": {
      "title": "...",
      "description": "...",
      "image": "...",
      "embed": { ... }
    }
  }
}
```

### Where to Publish

The event should be published from `MetadataWorker.processJob()` after successfully updating the database.

### Channel Strategy

Options for which channel to publish to:
1. **Post-specific channel**: `post:{post_id}` - Only users viewing that post receive it
2. **Section channel**: `section:{section_id}` - All users in the section receive it
3. **Global channel**: `global` - All connected users receive it

**Recommendation**: Use section channel since posts are typically viewed in section feeds. This requires looking up the post's section_id.

## Files to Modify

| File | Changes |
|------|---------|
| `backend/internal/handlers/pubsub.go` | Add event struct and publish function |
| `backend/internal/services/metadata_worker.go` | Call publish after successful fetch |

## Expected Outcomes

1. `link_metadata_updated` event is published after metadata fetch
2. Event contains post_id, link_id, and full metadata
3. Event is published to appropriate channel
4. Existing WebSocket infrastructure handles delivery
5. All tests pass

## Implementation

### pubsub.go additions

```go
// linkMetadataUpdatedData is the payload for link_metadata_updated events
type linkMetadataUpdatedData struct {
    PostID   uuid.UUID              `json:"post_id"`
    LinkID   uuid.UUID              `json:"link_id"`
    Metadata map[string]interface{} `json:"metadata"`
}

// PublishLinkMetadataUpdated publishes a link metadata update event
func PublishLinkMetadataUpdated(ctx context.Context, rdb *redis.Client, sectionID, postID, linkID uuid.UUID, metadata map[string]interface{}) error {
    data := linkMetadataUpdatedData{
        PostID:   postID,
        LinkID:   linkID,
        Metadata: metadata,
    }

    // Publish to section channel so all users viewing the section see the update
    channel := formatChannel(sectionPrefix, sectionID)
    return publishEvent(ctx, rdb, channel, "link_metadata_updated", data)
}
```

### metadata_worker.go modifications

```go
// Add to MetadataWorker struct
type MetadataWorker struct {
    redis       *redis.Client
    db          *sql.DB
    linkService *LinkMetadataService
    workerCount int
    stopCh      chan struct{}
    wg          sync.WaitGroup
}

// Update processJob to publish WebSocket event
func (w *MetadataWorker) processJob(ctx context.Context, job *MetadataJob, workerID int) {
    // ... existing fetch and update logic ...

    // After successful database update:

    // Look up section_id for the post
    sectionID, err := w.getPostSectionID(ctx, job.PostID)
    if err != nil {
        observability.LogWarn(ctx, "failed to get section_id for websocket event",
            "post_id", job.PostID, "error", err)
        // Still acknowledge job, just skip WebSocket
        AckMetadataJob(ctx, w.redis, *job)
        return
    }

    // Publish WebSocket event
    if err := handlers.PublishLinkMetadataUpdated(ctx, w.redis, sectionID, job.PostID, job.LinkID, metadata); err != nil {
        observability.LogWarn(ctx, "failed to publish metadata update event",
            "post_id", job.PostID, "link_id", job.LinkID, "error", err)
        // Don't fail - metadata is already saved
    }

    observability.LogInfo(ctx, "metadata fetched and event published",
        "worker_id", workerID,
        "post_id", job.PostID,
        "link_id", job.LinkID)

    AckMetadataJob(ctx, w.redis, *job)
}

func (w *MetadataWorker) getPostSectionID(ctx context.Context, postID uuid.UUID) (uuid.UUID, error) {
    var sectionID uuid.UUID
    err := w.db.QueryRowContext(ctx,
        "SELECT section_id FROM posts WHERE id = $1",
        postID,
    ).Scan(&sectionID)
    return sectionID, err
}
```

## Test Cases

```go
func TestPublishLinkMetadataUpdated(t *testing.T) {
    rdb := setupTestRedis(t)
    ctx := context.Background()

    sectionID := uuid.New()
    postID := uuid.New()
    linkID := uuid.New()
    metadata := map[string]interface{}{
        "title":       "Test Song",
        "description": "A great song",
        "image":       "https://example.com/image.jpg",
    }

    // Subscribe to the channel before publishing
    channel := formatChannel(sectionPrefix, sectionID)
    pubsub := rdb.Subscribe(ctx, channel)
    defer pubsub.Close()

    // Wait for subscription to be ready
    _, err := pubsub.Receive(ctx)
    require.NoError(t, err)

    // Publish the event
    err = PublishLinkMetadataUpdated(ctx, rdb, sectionID, postID, linkID, metadata)
    require.NoError(t, err)

    // Receive the message
    msg, err := pubsub.ReceiveMessage(ctx)
    require.NoError(t, err)

    // Parse and verify
    var event struct {
        Type string                   `json:"type"`
        Data linkMetadataUpdatedData `json:"data"`
    }
    err = json.Unmarshal([]byte(msg.Payload), &event)
    require.NoError(t, err)

    assert.Equal(t, "link_metadata_updated", event.Type)
    assert.Equal(t, postID, event.Data.PostID)
    assert.Equal(t, linkID, event.Data.LinkID)
    assert.Equal(t, "Test Song", event.Data.Metadata["title"])
}

func TestMetadataWorker_PublishesWebSocketEvent(t *testing.T) {
    rdb := setupTestRedis(t)
    db, mock, _ := sqlmock.New()
    defer db.Close()

    sectionID := uuid.New()
    postID := uuid.New()
    linkID := uuid.New()

    // Mock link service
    linkService := NewMockLinkMetadataService(map[string]interface{}{
        "title": "Test",
    })

    // Expect database update
    mock.ExpectExec("UPDATE links").WillReturnResult(sqlmock.NewResult(0, 1))

    // Expect section_id lookup
    mock.ExpectQuery("SELECT section_id FROM posts").
        WithArgs(postID).
        WillReturnRows(sqlmock.NewRows([]string{"section_id"}).AddRow(sectionID))

    worker := NewMetadataWorker(rdb, db, linkService, 1)

    ctx := context.Background()

    // Subscribe to section channel
    channel := formatChannel(sectionPrefix, sectionID)
    pubsub := rdb.Subscribe(ctx, channel)
    defer pubsub.Close()
    pubsub.Receive(ctx) // Wait for subscription

    worker.Start(ctx)
    defer worker.Stop(ctx)

    // Enqueue job
    job := MetadataJob{
        PostID:    postID,
        LinkID:    linkID,
        URL:       "https://example.com/test",
        CreatedAt: time.Now(),
    }
    EnqueueMetadataJob(ctx, rdb, job)

    // Wait for and verify WebSocket message
    msg, err := pubsub.ReceiveMessage(ctx)
    require.NoError(t, err)
    assert.Contains(t, msg.Payload, "link_metadata_updated")
}
```

## Verification

```bash
# Run pubsub tests
cd backend && go test ./internal/handlers/pubsub_test.go -v

# Run worker tests
cd backend && go test ./internal/services/metadata_worker_test.go -v

# Manual verification:
# 1. Start the app with dev server
# 2. Open browser dev tools, Network tab, filter by WS
# 3. Create a post with a Bandcamp link
# 4. Watch for link_metadata_updated message in WebSocket
```

## Notes

- Verify the existing pub/sub infrastructure in `pubsub.go`
- Check how other events (post_created, comment_added) are published for consistency
- The section channel approach means users need to be subscribed to the section
- If posts can be in multiple sections or users view cross-section feeds, consider additional channels
- The frontend handler for this event will be implemented in the next step
