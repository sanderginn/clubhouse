package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMetadataQueueTestRedis(t *testing.T) *redis.Client {
	client := testutil.GetTestRedis(t)

	ctx := context.Background()
	client.Del(ctx, MetadataQueueKey, MetadataQueueProcessingKey)

	t.Cleanup(func() {
		client.Del(ctx, MetadataQueueKey, MetadataQueueProcessingKey)
		testutil.CleanupRedis(t)
	})

	return client
}

func TestEnqueueMetadataJob(t *testing.T) {
	rdb := setupMetadataQueueTestRedis(t)
	ctx := context.Background()

	job := MetadataJob{
		PostID:    uuid.New(),
		LinkID:    uuid.New(),
		URL:       "https://example.com/test",
		CreatedAt: time.Now(),
	}

	err := EnqueueMetadataJob(ctx, rdb, job)
	require.NoError(t, err)

	length, err := GetQueueLength(ctx, rdb)
	require.NoError(t, err)
	assert.Equal(t, int64(1), length)
}

func TestDequeueMetadataJob(t *testing.T) {
	rdb := setupMetadataQueueTestRedis(t)
	ctx := context.Background()

	originalJob := MetadataJob{
		PostID:    uuid.New(),
		LinkID:    uuid.New(),
		URL:       "https://example.com/test",
		CreatedAt: time.Now().Truncate(time.Second),
	}
	err := EnqueueMetadataJob(ctx, rdb, originalJob)
	require.NoError(t, err)

	job, err := DequeueMetadataJob(ctx, rdb, 1*time.Second)
	require.NoError(t, err)
	require.NotNil(t, job)

	assert.Equal(t, originalJob.PostID, job.PostID)
	assert.Equal(t, originalJob.LinkID, job.LinkID)
	assert.Equal(t, originalJob.URL, job.URL)

	queueLen, _ := GetQueueLength(ctx, rdb)
	processingLen, _ := GetProcessingLength(ctx, rdb)
	assert.Equal(t, int64(0), queueLen)
	assert.Equal(t, int64(1), processingLen)
}

func TestDequeueMetadataJob_Timeout(t *testing.T) {
	rdb := setupMetadataQueueTestRedis(t)
	ctx := context.Background()

	// Use 1 second timeout as miniredis requires minimum 1s for blocking operations
	job, err := DequeueMetadataJob(ctx, rdb, 1*time.Second)
	require.NoError(t, err)
	assert.Nil(t, job)
}

func TestAckMetadataJob(t *testing.T) {
	rdb := setupMetadataQueueTestRedis(t)
	ctx := context.Background()

	job := MetadataJob{
		PostID:    uuid.New(),
		LinkID:    uuid.New(),
		URL:       "https://example.com/test",
		CreatedAt: time.Now(),
	}
	err := EnqueueMetadataJob(ctx, rdb, job)
	require.NoError(t, err)

	dequeuedJob, err := DequeueMetadataJob(ctx, rdb, 1*time.Second)
	require.NoError(t, err)
	require.NotNil(t, dequeuedJob)

	err = AckMetadataJob(ctx, rdb, *dequeuedJob)
	require.NoError(t, err)

	processingLen, _ := GetProcessingLength(ctx, rdb)
	assert.Equal(t, int64(0), processingLen)
}

func TestRequeueProcessingJobs(t *testing.T) {
	rdb := setupMetadataQueueTestRedis(t)
	ctx := context.Background()

	job1 := MetadataJob{PostID: uuid.New(), LinkID: uuid.New(), URL: "https://example.com/1", CreatedAt: time.Now()}
	job2 := MetadataJob{PostID: uuid.New(), LinkID: uuid.New(), URL: "https://example.com/2", CreatedAt: time.Now()}

	require.NoError(t, EnqueueMetadataJob(ctx, rdb, job1))
	require.NoError(t, EnqueueMetadataJob(ctx, rdb, job2))

	dequeued1, err := DequeueMetadataJob(ctx, rdb, 1*time.Second)
	require.NoError(t, err)
	require.NotNil(t, dequeued1)
	dequeued2, err := DequeueMetadataJob(ctx, rdb, 1*time.Second)
	require.NoError(t, err)
	require.NotNil(t, dequeued2)

	requeued, err := RequeueProcessingJobs(ctx, rdb)
	require.NoError(t, err)
	assert.Equal(t, 2, requeued)

	queueLen, _ := GetQueueLength(ctx, rdb)
	processingLen, _ := GetProcessingLength(ctx, rdb)
	assert.Equal(t, int64(2), queueLen)
	assert.Equal(t, int64(0), processingLen)

	redo1, err := DequeueMetadataJob(ctx, rdb, 1*time.Second)
	require.NoError(t, err)
	redo2, err := DequeueMetadataJob(ctx, rdb, 1*time.Second)
	require.NoError(t, err)

	assert.Equal(t, job1.URL, redo1.URL)
	assert.Equal(t, job2.URL, redo2.URL)
}

func TestDequeueMetadataJob_FIFO(t *testing.T) {
	rdb := setupMetadataQueueTestRedis(t)
	ctx := context.Background()

	job1 := MetadataJob{PostID: uuid.New(), LinkID: uuid.New(), URL: "https://example.com/1", CreatedAt: time.Now()}
	job2 := MetadataJob{PostID: uuid.New(), LinkID: uuid.New(), URL: "https://example.com/2", CreatedAt: time.Now()}
	job3 := MetadataJob{PostID: uuid.New(), LinkID: uuid.New(), URL: "https://example.com/3", CreatedAt: time.Now()}

	err := EnqueueMetadataJob(ctx, rdb, job1)
	require.NoError(t, err)
	err = EnqueueMetadataJob(ctx, rdb, job2)
	require.NoError(t, err)
	err = EnqueueMetadataJob(ctx, rdb, job3)
	require.NoError(t, err)

	dequeued1, err := DequeueMetadataJob(ctx, rdb, 1*time.Second)
	require.NoError(t, err)
	dequeued2, err := DequeueMetadataJob(ctx, rdb, 1*time.Second)
	require.NoError(t, err)
	dequeued3, err := DequeueMetadataJob(ctx, rdb, 1*time.Second)
	require.NoError(t, err)

	assert.Equal(t, job1.URL, dequeued1.URL)
	assert.Equal(t, job2.URL, dequeued2.URL)
	assert.Equal(t, job3.URL, dequeued3.URL)
}

func TestEnqueueMetadataJob_MultipleJobs(t *testing.T) {
	rdb := setupMetadataQueueTestRedis(t)
	ctx := context.Background()

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
