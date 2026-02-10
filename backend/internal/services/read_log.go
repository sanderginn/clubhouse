package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/sanderginn/clubhouse/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const (
	readLogLegacyCursorLayout = "2006-01-02T15:04:05.000Z07:00"
	readLogCursorSeparator    = "|"
)

// ReadLogService handles read log operations for book posts.
type ReadLogService struct {
	db           *sql.DB
	auditService *AuditService
}

// NewReadLogService creates a new read log service.
func NewReadLogService(db *sql.DB) *ReadLogService {
	return &ReadLogService{
		db:           db,
		auditService: NewAuditService(db),
	}
}

// LogRead creates or restores a read log for a book post.
func (s *ReadLogService) LogRead(ctx context.Context, userID, postID uuid.UUID, rating *int) (*models.ReadLog, error) {
	ctx, span := otel.Tracer("clubhouse.read_logs").Start(ctx, "ReadLogService.LogRead")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
		attribute.Bool("has_rating", rating != nil),
	)
	if rating != nil {
		span.SetAttributes(attribute.Int("rating", *rating))
	}
	defer span.End()

	if err := validateReadLogRating(rating); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if err := s.verifyReadablePost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	existing, err := s.getExistingReadLog(ctx, userID, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if existing != nil {
		if existing.DeletedAt != nil {
			readLog, err := s.restoreReadLog(ctx, existing.ID, rating)
			if err != nil {
				recordSpanError(span, err)
				return nil, err
			}
			if err := s.logReadAudit(ctx, "log_read", userID, buildReadLogMetadata(postID, rating, nil)); err != nil {
				recordSpanError(span, err)
				return nil, err
			}
			return readLog, nil
		}

		return existing, nil
	}

	readLog, err := s.createReadLog(ctx, userID, postID, rating)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if err := s.logReadAudit(ctx, "log_read", userID, buildReadLogMetadata(postID, rating, nil)); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return readLog, nil
}

// RemoveReadLog soft deletes an existing read log.
func (s *ReadLogService) RemoveReadLog(ctx context.Context, userID, postID uuid.UUID) error {
	ctx, span := otel.Tracer("clubhouse.read_logs").Start(ctx, "ReadLogService.RemoveReadLog")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
	)
	defer span.End()

	if err := s.verifyReadablePost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return err
	}

	query := `
		UPDATE read_logs
		SET deleted_at = now()
		WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL
	`

	result, err := s.db.ExecContext(ctx, query, userID, postID)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to remove read log: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	if rowsAffected == 0 {
		notFoundErr := errors.New("read log not found")
		recordSpanError(span, notFoundErr)
		return notFoundErr
	}

	if err := s.logReadAudit(ctx, "remove_read_log", userID, map[string]interface{}{"post_id": postID.String()}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

// UpdateRating updates the rating on an existing read log.
func (s *ReadLogService) UpdateRating(ctx context.Context, userID, postID uuid.UUID, rating int) (*models.ReadLog, error) {
	ctx, span := otel.Tracer("clubhouse.read_logs").Start(ctx, "ReadLogService.UpdateRating")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
		attribute.Int("rating", rating),
	)
	defer span.End()

	if err := validateReadLogRating(&rating); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if err := s.verifyReadablePost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	existing, err := s.getExistingReadLog(ctx, userID, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	if existing == nil || existing.DeletedAt != nil {
		notFoundErr := errors.New("read log not found")
		recordSpanError(span, notFoundErr)
		return nil, notFoundErr
	}

	updated, err := s.updateReadLogRating(ctx, existing.ID, rating)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if err := s.logReadAudit(ctx, "update_read_rating", userID, map[string]interface{}{
		"post_id":    postID.String(),
		"old_rating": existing.Rating,
		"new_rating": updated.Rating,
	}); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return updated, nil
}

// GetPostReadLogs returns read log summary and readers for a post.
func (s *ReadLogService) GetPostReadLogs(ctx context.Context, postID uuid.UUID, viewerID *uuid.UUID) (*models.PostReadLogsResponse, error) {
	ctx, span := otel.Tracer("clubhouse.read_logs").Start(ctx, "ReadLogService.GetPostReadLogs")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.Bool("has_viewer", viewerID != nil),
	)
	if viewerID != nil {
		span.SetAttributes(attribute.String("viewer_id", viewerID.String()))
	}
	defer span.End()

	if err := s.verifyReadablePost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	response := &models.PostReadLogsResponse{Readers: []models.ReadLogUserInfo{}}
	if err := s.populateReadLogSummaries(ctx, map[uuid.UUID]*models.PostReadLogsResponse{postID: response}); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if viewerID != nil {
		viewerLog, err := s.getViewerReadLog(ctx, postID, *viewerID)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		if viewerLog != nil {
			response.ViewerRead = true
			response.ViewerRating = viewerLog.Rating
		}
	}

	return response, nil
}

// GetReadLogsForPosts returns read log summaries keyed by post ID.
func (s *ReadLogService) GetReadLogsForPosts(ctx context.Context, postIDs []uuid.UUID, viewerID *uuid.UUID) (map[uuid.UUID]*models.PostReadLogsResponse, error) {
	ctx, span := otel.Tracer("clubhouse.read_logs").Start(ctx, "ReadLogService.GetReadLogsForPosts")
	span.SetAttributes(
		attribute.Int("post_count", len(postIDs)),
		attribute.Bool("has_viewer", viewerID != nil),
	)
	if viewerID != nil {
		span.SetAttributes(attribute.String("viewer_id", viewerID.String()))
	}
	defer span.End()

	responses := make(map[uuid.UUID]*models.PostReadLogsResponse, len(postIDs))
	for _, postID := range postIDs {
		if _, exists := responses[postID]; exists {
			continue
		}
		responses[postID] = &models.PostReadLogsResponse{Readers: []models.ReadLogUserInfo{}}
	}

	if len(responses) == 0 {
		return responses, nil
	}

	if err := s.populateReadLogSummaries(ctx, responses); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if viewerID != nil {
		orderedPostIDs := make([]uuid.UUID, 0, len(responses))
		for postID := range responses {
			orderedPostIDs = append(orderedPostIDs, postID)
		}

		rows, err := s.db.QueryContext(ctx, `
			SELECT post_id, rating
			FROM read_logs
			WHERE post_id = ANY($1) AND user_id = $2 AND deleted_at IS NULL
		`, pq.Array(orderedPostIDs), *viewerID)
		if err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to fetch viewer read logs: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var postID uuid.UUID
			var rating sql.NullInt64
			if err := rows.Scan(&postID, &rating); err != nil {
				recordSpanError(span, err)
				return nil, fmt.Errorf("failed to scan viewer read log: %w", err)
			}

			if response, ok := responses[postID]; ok {
				response.ViewerRead = true
				if rating.Valid {
					viewerRating := int(rating.Int64)
					response.ViewerRating = &viewerRating
				}
			}
		}
		if err := rows.Err(); err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to iterate viewer read logs: %w", err)
		}
	}

	return responses, nil
}

// GetUserReadHistory returns a paginated list of active read logs for a user.
func (s *ReadLogService) GetUserReadHistory(ctx context.Context, userID uuid.UUID, cursor *string, limit int) ([]models.ReadLog, *string, error) {
	ctx, span := otel.Tracer("clubhouse.read_logs").Start(ctx, "ReadLogService.GetUserReadHistory")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.Bool("has_cursor", cursor != nil && strings.TrimSpace(*cursor) != ""),
		attribute.Int("limit", limit),
	)
	defer span.End()

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var userExists bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM users
			WHERE id = $1 AND deleted_at IS NULL AND approved_at IS NOT NULL
		)
	`, userID).Scan(&userExists); err != nil {
		recordSpanError(span, err)
		return nil, nil, fmt.Errorf("failed to check user: %w", err)
	}
	if !userExists {
		notFoundErr := errors.New("user not found")
		recordSpanError(span, notFoundErr)
		return nil, nil, notFoundErr
	}

	query := `
		SELECT id, user_id, post_id, rating, created_at, deleted_at
		FROM read_logs
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	args := []interface{}{userID}
	argIndex := 2

	if cursor != nil && strings.TrimSpace(*cursor) != "" {
		cursorCreatedAt, cursorLogID, hasLogID, err := parseReadLogCursor(strings.TrimSpace(*cursor))
		if err != nil {
			recordSpanError(span, err)
			return nil, nil, err
		}

		if hasLogID {
			query += fmt.Sprintf(" AND (created_at < $%d OR (created_at = $%d AND id < $%d))", argIndex, argIndex, argIndex+1)
			args = append(args, cursorCreatedAt, cursorLogID)
			argIndex += 2
		} else {
			// Backward compatibility for existing timestamp-only cursors.
			query += fmt.Sprintf(" AND created_at < $%d", argIndex)
			args = append(args, cursorCreatedAt)
			argIndex++
		}
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", argIndex)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		recordSpanError(span, err)
		return nil, nil, fmt.Errorf("failed to query read history: %w", err)
	}
	defer rows.Close()

	history := make([]models.ReadLog, 0, limit)
	for rows.Next() {
		entry, err := scanReadLog(rows)
		if err != nil {
			recordSpanError(span, err)
			return nil, nil, err
		}
		history = append(history, *entry)
	}
	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, nil, fmt.Errorf("failed to iterate read history: %w", err)
	}

	hasMore := len(history) > limit
	if hasMore {
		history = history[:limit]
	}

	var nextCursor *string
	if hasMore && len(history) > 0 {
		cursorValue := buildReadLogCursor(history[len(history)-1].CreatedAt, history[len(history)-1].ID)
		nextCursor = &cursorValue
	}

	return history, nextCursor, nil
}

func (s *ReadLogService) verifyReadablePost(ctx context.Context, postID uuid.UUID) error {
	var sectionType string
	query := `
		SELECT s.type
		FROM posts p
		JOIN sections s ON p.section_id = s.id
		WHERE p.id = $1 AND p.deleted_at IS NULL
	`
	if err := s.db.QueryRowContext(ctx, query, postID).Scan(&sectionType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("post not found")
		}
		return fmt.Errorf("failed to verify readable post: %w", err)
	}
	if sectionType != "book" {
		return errors.New("post is not a book")
	}
	return nil
}

func (s *ReadLogService) getExistingReadLog(ctx context.Context, userID, postID uuid.UUID) (*models.ReadLog, error) {
	query := `
		SELECT id, user_id, post_id, rating, created_at, deleted_at
		FROM read_logs
		WHERE user_id = $1 AND post_id = $2
		ORDER BY deleted_at NULLS FIRST, created_at DESC
		LIMIT 1
	`

	row := s.db.QueryRowContext(ctx, query, userID, postID)
	readLog, err := scanReadLog(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch existing read log: %w", err)
	}

	return readLog, nil
}

func (s *ReadLogService) createReadLog(ctx context.Context, userID, postID uuid.UUID, rating *int) (*models.ReadLog, error) {
	query := `
		INSERT INTO read_logs (id, user_id, post_id, rating, created_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING id, user_id, post_id, rating, created_at, deleted_at
	`

	readLogID := uuid.New()
	row := s.db.QueryRowContext(ctx, query, readLogID, userID, postID, ratingToDBValue(rating))
	readLog, err := scanReadLog(row)
	if err != nil {
		return nil, fmt.Errorf("failed to create read log: %w", err)
	}

	return readLog, nil
}

func (s *ReadLogService) restoreReadLog(ctx context.Context, readLogID uuid.UUID, rating *int) (*models.ReadLog, error) {
	query := `
		UPDATE read_logs
		SET deleted_at = NULL,
			rating = $2,
			created_at = now()
		WHERE id = $1
		RETURNING id, user_id, post_id, rating, created_at, deleted_at
	`

	row := s.db.QueryRowContext(ctx, query, readLogID, ratingToDBValue(rating))
	readLog, err := scanReadLog(row)
	if err != nil {
		return nil, fmt.Errorf("failed to restore read log: %w", err)
	}

	return readLog, nil
}

func (s *ReadLogService) updateReadLogRating(ctx context.Context, readLogID uuid.UUID, rating int) (*models.ReadLog, error) {
	query := `
		UPDATE read_logs
		SET rating = $2
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, user_id, post_id, rating, created_at, deleted_at
	`

	row := s.db.QueryRowContext(ctx, query, readLogID, rating)
	readLog, err := scanReadLog(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("read log not found")
		}
		return nil, fmt.Errorf("failed to update read rating: %w", err)
	}

	return readLog, nil
}

func (s *ReadLogService) getViewerReadLog(ctx context.Context, postID, viewerID uuid.UUID) (*models.ReadLog, error) {
	query := `
		SELECT id, user_id, post_id, rating, created_at, deleted_at
		FROM read_logs
		WHERE post_id = $1 AND user_id = $2 AND deleted_at IS NULL
	`

	row := s.db.QueryRowContext(ctx, query, postID, viewerID)
	readLog, err := scanReadLog(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch viewer read log: %w", err)
	}

	return readLog, nil
}

func (s *ReadLogService) populateReadLogSummaries(ctx context.Context, responses map[uuid.UUID]*models.PostReadLogsResponse) error {
	postIDs := make([]uuid.UUID, 0, len(responses))
	for postID := range responses {
		postIDs = append(postIDs, postID)
	}

	summaryRows, err := s.db.QueryContext(ctx, `
		SELECT post_id, COUNT(*) AS read_count, ROUND(AVG(rating)::numeric, 1) AS average_rating
		FROM read_logs
		WHERE post_id = ANY($1) AND deleted_at IS NULL
		GROUP BY post_id
	`, pq.Array(postIDs))
	if err != nil {
		return fmt.Errorf("failed to fetch read log summary: %w", err)
	}
	defer summaryRows.Close()

	for summaryRows.Next() {
		var postID uuid.UUID
		var readCount int
		var avgRating sql.NullFloat64
		if err := summaryRows.Scan(&postID, &readCount, &avgRating); err != nil {
			return fmt.Errorf("failed to scan read log summary: %w", err)
		}

		if response, ok := responses[postID]; ok {
			response.ReadCount = readCount
			if avgRating.Valid {
				response.AverageRating = avgRating.Float64
			}
		}
	}
	if err := summaryRows.Err(); err != nil {
		return fmt.Errorf("failed to iterate read log summary: %w", err)
	}

	readerRows, err := s.db.QueryContext(ctx, `
		SELECT rl.post_id, u.id, u.username, u.profile_picture_url, rl.rating
		FROM read_logs rl
		JOIN users u ON rl.user_id = u.id
		WHERE rl.post_id = ANY($1) AND rl.deleted_at IS NULL
		ORDER BY rl.post_id ASC, rl.created_at DESC, rl.id DESC
	`, pq.Array(postIDs))
	if err != nil {
		return fmt.Errorf("failed to fetch read log readers: %w", err)
	}
	defer readerRows.Close()

	for readerRows.Next() {
		var postID uuid.UUID
		var reader models.ReadLogUserInfo
		var rating sql.NullInt64
		if err := readerRows.Scan(&postID, &reader.ID, &reader.Username, &reader.ProfilePictureUrl, &rating); err != nil {
			return fmt.Errorf("failed to scan read log reader: %w", err)
		}
		if rating.Valid {
			ratingValue := int(rating.Int64)
			reader.Rating = &ratingValue
		}

		if response, ok := responses[postID]; ok {
			response.Readers = append(response.Readers, reader)
		}
	}
	if err := readerRows.Err(); err != nil {
		return fmt.Errorf("failed to iterate read log readers: %w", err)
	}

	return nil
}

func (s *ReadLogService) logReadAudit(ctx context.Context, action string, userID uuid.UUID, metadata map[string]interface{}) error {
	if err := s.auditService.LogAuditWithMetadata(ctx, action, uuid.Nil, userID, metadata); err != nil {
		return fmt.Errorf("failed to create read log audit log: %w", err)
	}
	return nil
}

func validateReadLogRating(rating *int) error {
	if rating == nil {
		return nil
	}
	if *rating < 1 || *rating > 5 {
		return errors.New("rating must be between 1 and 5")
	}
	return nil
}

func buildReadLogMetadata(postID uuid.UUID, rating *int, extra map[string]interface{}) map[string]interface{} {
	metadata := map[string]interface{}{"post_id": postID.String()}
	if rating != nil {
		metadata["rating"] = *rating
	}
	for key, value := range extra {
		metadata[key] = value
	}
	return metadata
}

func ratingToDBValue(rating *int) interface{} {
	if rating == nil {
		return nil
	}
	return *rating
}

func scanReadLog(scanner interface {
	Scan(dest ...interface{}) error
}) (*models.ReadLog, error) {
	var readLog models.ReadLog
	var rating sql.NullInt64
	var deletedAt sql.NullTime
	if err := scanner.Scan(&readLog.ID, &readLog.UserID, &readLog.PostID, &rating, &readLog.CreatedAt, &deletedAt); err != nil {
		return nil, err
	}

	if rating.Valid {
		ratingValue := int(rating.Int64)
		readLog.Rating = &ratingValue
	}
	if deletedAt.Valid {
		readLog.DeletedAt = &deletedAt.Time
	}

	return &readLog, nil
}

func buildReadLogCursor(createdAt time.Time, readLogID uuid.UUID) string {
	return createdAt.UTC().Format(time.RFC3339Nano) + readLogCursorSeparator + readLogID.String()
}

func parseReadLogCursor(cursor string) (time.Time, uuid.UUID, bool, error) {
	parts := strings.Split(cursor, readLogCursorSeparator)
	if len(parts) == 2 {
		createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return time.Time{}, uuid.Nil, false, errors.New("invalid cursor")
		}

		readLogID, err := uuid.Parse(parts[1])
		if err != nil {
			return time.Time{}, uuid.Nil, false, errors.New("invalid cursor")
		}

		return createdAt.UTC(), readLogID, true, nil
	}

	// Backward compatibility for existing timestamp-only cursors.
	createdAt, err := time.Parse(readLogLegacyCursorLayout, cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, false, errors.New("invalid cursor")
	}

	return createdAt.UTC(), uuid.Nil, false, nil
}
