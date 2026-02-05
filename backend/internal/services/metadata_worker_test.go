package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockMetadataFetcher struct {
	metadata map[string]interface{}
	err      error
	called   int
	urls     []string
}

func (m *mockMetadataFetcher) Fetch(_ context.Context, url string) (map[string]interface{}, error) {
	m.called++
	m.urls = append(m.urls, url)
	return m.metadata, m.err
}

func setupMetadataWorkerTestRedis(t *testing.T) *redis.Client {
	client := testutil.GetTestRedis(t)
	ctx := context.Background()
	client.Del(ctx, MetadataQueueKey, MetadataQueueProcessingKey)

	t.Cleanup(func() {
		client.Del(ctx, MetadataQueueKey, MetadataQueueProcessingKey)
		testutil.CleanupRedis(t)
	})

	return client
}

func setupMetadataWorkerTestDB(t *testing.T) *sql.DB {
	db := testutil.RequireTestDB(t)
	testutil.CleanupTables(t, db)
	t.Cleanup(func() {
		testutil.CleanupTables(t, db)
	})
	return db
}

func createTestLink(t *testing.T, db *sql.DB, postID, url string) string {
	t.Helper()
	var id string
	query := `INSERT INTO links (id, post_id, url, created_at) VALUES (gen_random_uuid(), $1, $2, now()) RETURNING id`
	err := db.QueryRow(query, postID, url).Scan(&id)
	require.NoError(t, err)
	return id
}

func TestNewMetadataWorker(t *testing.T) {
	rdb := setupMetadataWorkerTestRedis(t)
	fetcher := &mockMetadataFetcher{}

	worker := NewMetadataWorker(rdb, nil, fetcher, 5)
	assert.Equal(t, 5, worker.workerCount)

	worker = NewMetadataWorker(rdb, nil, fetcher, 0)
	assert.Equal(t, 3, worker.workerCount)

	worker = NewMetadataWorker(rdb, nil, fetcher, -1)
	assert.Equal(t, 3, worker.workerCount)
}

func TestMetadataWorker_StartStop(t *testing.T) {
	rdb := setupMetadataWorkerTestRedis(t)
	fetcher := &mockMetadataFetcher{}

	worker := NewMetadataWorker(rdb, nil, fetcher, 2)

	ctx := context.Background()
	worker.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	worker.Stop(ctx)
}

func TestMetadataWorker_ProcessJob(t *testing.T) {
	rdb := setupMetadataWorkerTestRedis(t)
	db := setupMetadataWorkerTestDB(t)
	ctx := context.Background()

	userID := testutil.CreateTestUser(t, db, "testuser", "test@example.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Test Section", "music")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test post")
	linkID := createTestLink(t, db, postID, "https://example.com/test")

	fetcher := &mockMetadataFetcher{
		metadata: map[string]interface{}{
			"title":       "Test Title",
			"description": "Test Description",
		},
	}

	worker := NewMetadataWorker(rdb, db, fetcher, 1)

	channel := "section:" + sectionID
	pubsub := rdb.Subscribe(ctx, channel)
	defer pubsub.Close()
	_, err := pubsub.Receive(ctx)
	require.NoError(t, err)

	job := MetadataJob{
		PostID:    uuid.MustParse(postID),
		LinkID:    uuid.MustParse(linkID),
		URL:       "https://example.com/test",
		CreatedAt: time.Now(),
	}
	err = EnqueueMetadataJob(ctx, rdb, job)
	require.NoError(t, err)

	worker.Start(ctx)

	time.Sleep(2 * time.Second)

	worker.Stop(ctx)

	assert.Equal(t, 1, fetcher.called)
	assert.Equal(t, []string{"https://example.com/test"}, fetcher.urls)

	receiveCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	msg, err := pubsub.ReceiveMessage(receiveCtx)
	require.NoError(t, err)

	var event struct {
		Type string `json:"type"`
		Data struct {
			PostID   string                 `json:"post_id"`
			LinkID   string                 `json:"link_id"`
			URL      string                 `json:"url"`
			Metadata map[string]interface{} `json:"metadata"`
		} `json:"data"`
	}
	err = json.Unmarshal([]byte(msg.Payload), &event)
	require.NoError(t, err)
	assert.Equal(t, "link_metadata_updated", event.Type)
	assert.Equal(t, postID, event.Data.PostID)
	assert.Equal(t, linkID, event.Data.LinkID)
	assert.Equal(t, "https://example.com/test", event.Data.URL)
	assert.Equal(t, "Test Title", event.Data.Metadata["title"])
	assert.Equal(t, "Test Description", event.Data.Metadata["description"])

	var metadata sql.NullString
	var updatedAt sql.NullTime
	err = db.QueryRow("SELECT metadata, updated_at FROM links WHERE id = $1", linkID).Scan(&metadata, &updatedAt)
	require.NoError(t, err)
	assert.True(t, metadata.Valid)
	assert.True(t, updatedAt.Valid)

	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(metadata.String), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "Test Title", parsed["title"])
	assert.Equal(t, "Test Description", parsed["description"])

	queueLen, _ := GetQueueLength(ctx, rdb)
	processingLen, _ := GetProcessingLength(ctx, rdb)
	assert.Equal(t, int64(0), queueLen)
	assert.Equal(t, int64(0), processingLen)
}

func TestMetadataWorker_ProcessMultipleJobs(t *testing.T) {
	rdb := setupMetadataWorkerTestRedis(t)
	db := setupMetadataWorkerTestDB(t)
	ctx := context.Background()

	userID := testutil.CreateTestUser(t, db, "testuser", "test@example.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Test Section", "music")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test post")

	linkID1 := createTestLink(t, db, postID, "https://example.com/1")
	linkID2 := createTestLink(t, db, postID, "https://example.com/2")
	linkID3 := createTestLink(t, db, postID, "https://example.com/3")

	fetcher := &mockMetadataFetcher{
		metadata: map[string]interface{}{"title": "Fetched"},
	}

	worker := NewMetadataWorker(rdb, db, fetcher, 2)

	for i, linkID := range []string{linkID1, linkID2, linkID3} {
		job := MetadataJob{
			PostID:    uuid.MustParse(postID),
			LinkID:    uuid.MustParse(linkID),
			URL:       "https://example.com/" + string(rune('1'+i)),
			CreatedAt: time.Now(),
		}
		err := EnqueueMetadataJob(ctx, rdb, job)
		require.NoError(t, err)
	}

	worker.Start(ctx)

	time.Sleep(4 * time.Second)

	worker.Stop(ctx)

	assert.Equal(t, 3, fetcher.called)

	for _, linkID := range []string{linkID1, linkID2, linkID3} {
		var metadata sql.NullString
		err := db.QueryRow("SELECT metadata FROM links WHERE id = $1", linkID).Scan(&metadata)
		require.NoError(t, err)
		assert.True(t, metadata.Valid)
	}

	queueLen, _ := GetQueueLength(ctx, rdb)
	processingLen, _ := GetProcessingLength(ctx, rdb)
	assert.Equal(t, int64(0), queueLen)
	assert.Equal(t, int64(0), processingLen)
}

func TestMetadataWorker_PreservesHighlights(t *testing.T) {
	rdb := setupMetadataWorkerTestRedis(t)
	db := setupMetadataWorkerTestDB(t)
	ctx := context.Background()

	userID := testutil.CreateTestUser(t, db, "testuser", "test@example.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Test Section", "music")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test post")
	linkID := createTestLink(t, db, postID, "https://example.com/test")

	highlights := []models.Highlight{
		{Timestamp: 12, Label: "Intro"},
		{Timestamp: 42, Label: "Drop"},
	}
	highlightsPayload, err := json.Marshal(map[string]interface{}{"highlights": highlights})
	require.NoError(t, err)

	_, err = db.Exec(`UPDATE links SET metadata = $1 WHERE id = $2`, highlightsPayload, linkID)
	require.NoError(t, err)

	fetcher := &mockMetadataFetcher{
		metadata: map[string]interface{}{
			"title":       "Test Title",
			"description": "Test Description",
		},
	}

	worker := NewMetadataWorker(rdb, db, fetcher, 1)

	job := MetadataJob{
		PostID:    uuid.MustParse(postID),
		LinkID:    uuid.MustParse(linkID),
		URL:       "https://example.com/test",
		CreatedAt: time.Now(),
	}
	err = EnqueueMetadataJob(ctx, rdb, job)
	require.NoError(t, err)

	worker.Start(ctx)
	time.Sleep(2 * time.Second)
	worker.Stop(ctx)

	var metadata sql.NullString
	err = db.QueryRow("SELECT metadata FROM links WHERE id = $1", linkID).Scan(&metadata)
	require.NoError(t, err)
	assert.True(t, metadata.Valid)

	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(metadata.String), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "Test Title", parsed["title"])
	assert.Equal(t, "Test Description", parsed["description"])

	storedHighlights, err := extractHighlightsFromMetadata(parsed)
	require.NoError(t, err)
	require.Len(t, storedHighlights, 2)
	assert.Equal(t, 12, storedHighlights[0].Timestamp)
	assert.Equal(t, "Intro", storedHighlights[0].Label)
	assert.Equal(t, 42, storedHighlights[1].Timestamp)
	assert.Equal(t, "Drop", storedHighlights[1].Label)
}

func TestMetadataWorker_FetchError(t *testing.T) {
	rdb := setupMetadataWorkerTestRedis(t)
	db := setupMetadataWorkerTestDB(t)
	ctx := context.Background()

	userID := testutil.CreateTestUser(t, db, "testuser", "test@example.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Test Section", "music")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test post")
	linkID := createTestLink(t, db, postID, "https://example.com/failing")

	fetcher := &mockMetadataFetcher{
		err: assert.AnError,
	}

	worker := NewMetadataWorker(rdb, db, fetcher, 1)

	job := MetadataJob{
		PostID:    uuid.MustParse(postID),
		LinkID:    uuid.MustParse(linkID),
		URL:       "https://example.com/failing",
		CreatedAt: time.Now(),
	}
	err := EnqueueMetadataJob(ctx, rdb, job)
	require.NoError(t, err)

	worker.Start(ctx)

	time.Sleep(2 * time.Second)

	worker.Stop(ctx)

	assert.Equal(t, 1, fetcher.called)

	var metadata sql.NullString
	err = db.QueryRow("SELECT metadata FROM links WHERE id = $1", linkID).Scan(&metadata)
	require.NoError(t, err)
	assert.False(t, metadata.Valid)

	queueLen, _ := GetQueueLength(ctx, rdb)
	processingLen, _ := GetProcessingLength(ctx, rdb)
	assert.Equal(t, int64(0), queueLen)
	assert.Equal(t, int64(0), processingLen)
}

func TestMetadataWorker_ContextCancellation(t *testing.T) {
	rdb := setupMetadataWorkerTestRedis(t)
	fetcher := &mockMetadataFetcher{}

	worker := NewMetadataWorker(rdb, nil, fetcher, 2)

	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	cancel()

	done := make(chan struct{})
	go func() {
		worker.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Error("workers did not stop after context cancellation")
	}
}

func TestMetadataWorker_GracefulShutdown(t *testing.T) {
	rdb := setupMetadataWorkerTestRedis(t)
	fetcher := &mockMetadataFetcher{}

	worker := NewMetadataWorker(rdb, nil, fetcher, 3)

	ctx := context.Background()
	worker.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		worker.Stop(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Error("worker stop timed out")
	}
}

func TestDefaultMetadataFetcher(t *testing.T) {
	fetcher := &DefaultMetadataFetcher{}
	assert.NotNil(t, fetcher)
}
