package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/observability"
	linkmeta "github.com/sanderginn/clubhouse/internal/services/links"
)

type linkMetadataUpdatedData struct {
	PostID   uuid.UUID              `json:"post_id"`
	LinkID   uuid.UUID              `json:"link_id"`
	Metadata map[string]interface{} `json:"metadata"`
}

// MetadataFetcher is an interface for fetching link metadata
type MetadataFetcher interface {
	Fetch(ctx context.Context, url string) (map[string]interface{}, error)
}

// MetadataWorker manages a pool of workers that process metadata fetch jobs
type MetadataWorker struct {
	redis       *redis.Client
	db          *sql.DB
	fetcher     MetadataFetcher
	workerCount int
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewMetadataWorker creates a new metadata worker pool
func NewMetadataWorker(rdb *redis.Client, db *sql.DB, fetcher MetadataFetcher, workerCount int) *MetadataWorker {
	if workerCount <= 0 {
		workerCount = 3
	}
	return &MetadataWorker{
		redis:       rdb,
		db:          db,
		fetcher:     fetcher,
		workerCount: workerCount,
		stopCh:      make(chan struct{}),
	}
}

// Start spawns the worker goroutines
func (w *MetadataWorker) Start(ctx context.Context) {
	observability.LogInfo(ctx, "starting metadata workers", "count", fmt.Sprintf("%d", w.workerCount))

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

	observability.LogInfo(ctx, "metadata worker started", "worker_id", fmt.Sprintf("%d", workerID))

	for {
		select {
		case <-w.stopCh:
			observability.LogInfo(ctx, "metadata worker stopping", "worker_id", fmt.Sprintf("%d", workerID))
			return
		case <-ctx.Done():
			observability.LogInfo(ctx, "metadata worker context cancelled", "worker_id", fmt.Sprintf("%d", workerID))
			return
		default:
		}

		job, err := DequeueMetadataJob(ctx, w.redis, 1*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				observability.LogInfo(ctx, "metadata worker context cancelled during dequeue", "worker_id", fmt.Sprintf("%d", workerID))
				return
			}
			observability.LogError(ctx, observability.ErrorLog{
				Message: "failed to dequeue metadata job",
				Code:    "METADATA_DEQUEUE_FAILED",
				Err:     err,
			})
			continue
		}

		if job == nil {
			continue
		}

		w.processJob(ctx, job, workerID)
	}
}

func (w *MetadataWorker) processJob(ctx context.Context, job *MetadataJob, workerID int) {
	observability.LogDebug(ctx, "processing metadata job",
		"worker_id", fmt.Sprintf("%d", workerID),
		"post_id", job.PostID.String(),
		"link_id", job.LinkID.String(),
		"url", job.URL)

	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	metadata, err := w.fetcher.Fetch(fetchCtx, job.URL)
	if err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message: "failed to fetch link metadata",
			Code:    "METADATA_FETCH_FAILED",
			Err:     err,
		})
		if ackErr := AckMetadataJob(ctx, w.redis, *job); ackErr != nil {
			observability.LogError(ctx, observability.ErrorLog{
				Message: "failed to acknowledge metadata job after fetch failure",
				Code:    "METADATA_ACK_FAILED",
				Err:     ackErr,
			})
		}
		return
	}

	if err := w.updateLinkMetadata(ctx, job.LinkID, metadata); err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message: "failed to update link metadata in database",
			Code:    "METADATA_UPDATE_FAILED",
			Err:     err,
		})
		if ackErr := AckMetadataJob(ctx, w.redis, *job); ackErr != nil {
			observability.LogError(ctx, observability.ErrorLog{
				Message: "failed to acknowledge metadata job after update failure",
				Code:    "METADATA_ACK_FAILED",
				Err:     ackErr,
			})
		}
		return
	}

	sectionID, err := w.getPostSectionID(ctx, job.PostID)
	if err != nil {
		observability.LogWarn(ctx, "failed to get section_id for metadata websocket event",
			"post_id", job.PostID.String(),
			"link_id", job.LinkID.String(),
			"error", err.Error(),
		)
	} else if err := w.publishLinkMetadataUpdated(ctx, sectionID, job.PostID, job.LinkID, metadata); err != nil {
		observability.LogWarn(ctx, "failed to publish metadata websocket event",
			"post_id", job.PostID.String(),
			"link_id", job.LinkID.String(),
			"error", err.Error(),
		)
	}

	observability.LogInfo(ctx, "metadata fetched and stored",
		"worker_id", fmt.Sprintf("%d", workerID),
		"post_id", job.PostID.String(),
		"link_id", job.LinkID.String())

	if err := AckMetadataJob(ctx, w.redis, *job); err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message: "failed to acknowledge completed metadata job",
			Code:    "METADATA_ACK_FAILED",
			Err:     err,
		})
	}
}

func (w *MetadataWorker) getPostSectionID(ctx context.Context, postID uuid.UUID) (uuid.UUID, error) {
	var sectionID uuid.UUID
	err := w.db.QueryRowContext(ctx, "SELECT section_id FROM posts WHERE id = $1", postID).Scan(&sectionID)
	if err != nil {
		return uuid.Nil, err
	}
	return sectionID, nil
}

func (w *MetadataWorker) publishLinkMetadataUpdated(ctx context.Context, sectionID uuid.UUID, postID uuid.UUID, linkID uuid.UUID, metadata map[string]interface{}) error {
	if w.redis == nil {
		return nil
	}

	payload, err := json.Marshal(realtimeEvent{
		Type: "link_metadata_updated",
		Data: linkMetadataUpdatedData{
			PostID:   postID,
			LinkID:   linkID,
			Metadata: metadata,
		},
		Timestamp: time.Now().UTC(),
	})
	if err != nil {
		return err
	}

	channel := fmt.Sprintf("section:%s", sectionID.String())
	if err := publishWithRetry(ctx, w.redis, channel, payload); err != nil {
		observability.RecordWebsocketError(ctx, "publish_failed", "link_metadata_updated")
		return err
	}
	return nil
}

func (w *MetadataWorker) updateLinkMetadata(ctx context.Context, linkID uuid.UUID, metadata map[string]interface{}) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	query := `UPDATE links SET metadata = $1, updated_at = NOW() WHERE id = $2`
	result, err := w.db.ExecContext(ctx, query, metadataJSON, linkID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("link not found: %s", linkID)
	}

	return nil
}

// DefaultMetadataFetcher wraps the links.FetchMetadata function
type DefaultMetadataFetcher struct{}

// Fetch implements MetadataFetcher using the links package
func (f *DefaultMetadataFetcher) Fetch(ctx context.Context, url string) (map[string]interface{}, error) {
	return linkmeta.FetchMetadata(ctx, url)
}
