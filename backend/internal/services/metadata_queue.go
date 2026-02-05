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
