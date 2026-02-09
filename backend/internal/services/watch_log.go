package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const (
	watchLogLegacyCursorLayout = "2006-01-02T15:04:05.000Z07:00"
	watchLogCursorSeparator    = "|"
)

// WatchLogServiceDependencies holds optional dependencies for WatchLogService.
type WatchLogServiceDependencies struct {
	AuditService *AuditService
	Now          func() time.Time
}

// WatchLogService handles watch log operations.
type WatchLogService struct {
	db           *sql.DB
	auditService *AuditService
	now          func() time.Time
}

// NewWatchLogService creates a new watch log service.
func NewWatchLogService(db *sql.DB, deps *WatchLogServiceDependencies) *WatchLogService {
	auditService := NewAuditService(db)
	now := time.Now
	if deps != nil {
		if deps.AuditService != nil {
			auditService = deps.AuditService
		}
		if deps.Now != nil {
			now = deps.Now
		}
	}

	return &WatchLogService{
		db:           db,
		auditService: auditService,
		now:          now,
	}
}

// LogWatch creates or restores a watch log for a movie or series post.
func (s *WatchLogService) LogWatch(ctx context.Context, userID, postID uuid.UUID, rating int, notes string) (*models.WatchLog, error) {
	ctx, span := otel.Tracer("clubhouse.watch_logs").Start(ctx, "WatchLogService.LogWatch")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
		attribute.Int("rating", rating),
		attribute.Bool("has_notes", strings.TrimSpace(notes) != ""),
	)
	defer span.End()

	if err := validateWatchLogRating(rating); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if err := s.verifyWatchablePost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	existing, err := s.getExistingWatchLog(ctx, userID, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if existing != nil {
		if existing.DeletedAt != nil {
			watchLog, err := s.restoreWatchLog(ctx, existing.ID, rating, notes)
			if err != nil {
				recordSpanError(span, err)
				return nil, err
			}

			if err := s.logWatchAudit(ctx, "log_watch", userID, map[string]interface{}{
				"post_id": postID.String(),
				"rating":  rating,
			}); err != nil {
				recordSpanError(span, err)
				return nil, err
			}
			return watchLog, nil
		}

		return existing, nil
	}

	watchLog, err := s.createWatchLog(ctx, userID, postID, rating, notes)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if err := s.logWatchAudit(ctx, "log_watch", userID, map[string]interface{}{
		"post_id": postID.String(),
		"rating":  rating,
	}); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return watchLog, nil
}

// UpdateWatchLog updates an existing watch log for a movie or series post.
func (s *WatchLogService) UpdateWatchLog(ctx context.Context, userID, postID uuid.UUID, rating *int, notes *string) (*models.WatchLog, error) {
	ctx, span := otel.Tracer("clubhouse.watch_logs").Start(ctx, "WatchLogService.UpdateWatchLog")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
		attribute.Bool("has_rating", rating != nil),
		attribute.Bool("has_notes", notes != nil),
	)
	if rating != nil {
		span.SetAttributes(attribute.Int("rating", *rating))
	}
	defer span.End()

	if rating == nil && notes == nil {
		err := errors.New("no fields to update")
		recordSpanError(span, err)
		return nil, err
	}

	if rating != nil {
		if err := validateWatchLogRating(*rating); err != nil {
			recordSpanError(span, err)
			return nil, err
		}
	}

	if err := s.verifyWatchablePost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	existing, err := s.getExistingWatchLog(ctx, userID, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	if existing == nil || existing.DeletedAt != nil {
		notFoundErr := errors.New("watch log not found")
		recordSpanError(span, notFoundErr)
		return nil, notFoundErr
	}

	updated, err := s.updateWatchLog(ctx, existing.ID, rating, notes)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	metadata := map[string]interface{}{
		"post_id": postID.String(),
	}
	if rating != nil {
		metadata["old_rating"] = existing.Rating
		metadata["new_rating"] = updated.Rating
	}
	if notes != nil {
		metadata["notes_updated"] = true
	}
	if err := s.logWatchAudit(ctx, "update_watch_log", userID, metadata); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return updated, nil
}

// RemoveWatchLog soft deletes a watch log for a movie or series post.
func (s *WatchLogService) RemoveWatchLog(ctx context.Context, userID, postID uuid.UUID) error {
	ctx, span := otel.Tracer("clubhouse.watch_logs").Start(ctx, "WatchLogService.RemoveWatchLog")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
	)
	defer span.End()

	if err := s.verifyWatchablePost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return err
	}

	query := `
		UPDATE watch_logs
		SET deleted_at = now(), updated_at = now()
		WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL
	`
	result, err := s.db.ExecContext(ctx, query, userID, postID)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to remove watch log: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	if rowsAffected == 0 {
		notFoundErr := errors.New("watch log not found")
		recordSpanError(span, notFoundErr)
		return notFoundErr
	}

	if err := s.logWatchAudit(ctx, "remove_watch_log", userID, map[string]interface{}{
		"post_id": postID.String(),
	}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

// GetUserWatchLogs retrieves paginated watch history for a user.
func (s *WatchLogService) GetUserWatchLogs(ctx context.Context, userID uuid.UUID, limit int, cursor *string) ([]models.WatchLogWithPost, *string, error) {
	ctx, span := otel.Tracer("clubhouse.watch_logs").Start(ctx, "WatchLogService.GetUserWatchLogs")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.Int("limit", limit),
		attribute.Bool("has_cursor", cursor != nil && *cursor != ""),
	)
	defer span.End()

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var exists bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL AND approved_at IS NOT NULL)
	`, userID).Scan(&exists); err != nil {
		recordSpanError(span, err)
		return nil, nil, fmt.Errorf("failed to check user: %w", err)
	}
	if !exists {
		notFoundErr := errors.New("user not found")
		recordSpanError(span, notFoundErr)
		return nil, nil, notFoundErr
	}

	query := `
		SELECT
			wl.id, wl.user_id, wl.post_id, wl.rating, wl.notes, wl.watched_at, wl.created_at, wl.updated_at, wl.deleted_at,
			p.id, p.user_id, p.section_id, p.content, p.created_at, p.updated_at, p.deleted_at, p.deleted_by_user_id
		FROM watch_logs wl
		JOIN posts p ON wl.post_id = p.id AND p.deleted_at IS NULL
		WHERE wl.user_id = $1 AND wl.deleted_at IS NULL
	`

	args := []interface{}{userID}
	argIndex := 2
	if cursor != nil && strings.TrimSpace(*cursor) != "" {
		cursorWatchedAt, cursorLogID, hasLogID, err := parseWatchLogCursor(strings.TrimSpace(*cursor))
		if err != nil {
			recordSpanError(span, err)
			return nil, nil, err
		}

		if hasLogID {
			query += fmt.Sprintf(" AND (wl.watched_at < $%d OR (wl.watched_at = $%d AND wl.id < $%d))", argIndex, argIndex, argIndex+1)
			args = append(args, cursorWatchedAt, cursorLogID)
			argIndex += 2
		} else {
			// Backward compatibility for legacy cursor format that only encoded watched_at.
			query += fmt.Sprintf(" AND wl.watched_at < $%d", argIndex)
			args = append(args, cursorWatchedAt)
			argIndex++
		}
	}

	query += fmt.Sprintf(" ORDER BY wl.watched_at DESC, wl.id DESC LIMIT $%d", argIndex)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		recordSpanError(span, err)
		return nil, nil, fmt.Errorf("failed to query watch logs: %w", err)
	}
	defer rows.Close()

	logs := make([]models.WatchLogWithPost, 0, limit)
	for rows.Next() {
		watchLog, post, err := scanWatchLogWithPost(rows)
		if err != nil {
			recordSpanError(span, err)
			return nil, nil, err
		}

		logs = append(logs, models.WatchLogWithPost{
			WatchLog: *watchLog,
			Post:     post,
		})
	}
	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, nil, fmt.Errorf("failed to iterate watch logs: %w", err)
	}

	hasMore := len(logs) > limit
	if hasMore {
		logs = logs[:limit]
	}

	var nextCursor *string
	if hasMore && len(logs) > 0 {
		cursorValue := buildWatchLogCursor(logs[len(logs)-1].WatchedAt, logs[len(logs)-1].ID)
		nextCursor = &cursorValue
	}

	return logs, nextCursor, nil
}

// GetPostWatchLogs retrieves watch log summary and entries for a post.
func (s *WatchLogService) GetPostWatchLogs(ctx context.Context, postID uuid.UUID, viewerID *uuid.UUID) (*models.PostWatchLogsResponse, error) {
	ctx, span := otel.Tracer("clubhouse.watch_logs").Start(ctx, "WatchLogService.GetPostWatchLogs")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.Bool("has_viewer", viewerID != nil),
	)
	defer span.End()

	if err := s.verifyWatchablePost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	var watchCount int
	var avgRating sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), AVG(rating)
		FROM watch_logs
		WHERE post_id = $1 AND deleted_at IS NULL
	`, postID).Scan(&watchCount, &avgRating); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to fetch watch log summary: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			wl.id, wl.user_id, wl.post_id, wl.rating, wl.notes, wl.watched_at, wl.created_at, wl.updated_at, wl.deleted_at,
			u.id, u.username, u.profile_picture_url
		FROM watch_logs wl
		JOIN users u ON wl.user_id = u.id
		WHERE wl.post_id = $1 AND wl.deleted_at IS NULL
		ORDER BY wl.watched_at DESC, wl.id DESC
	`, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to query post watch logs: %w", err)
	}
	defer rows.Close()

	logs := make([]models.WatchLogResponse, 0)
	for rows.Next() {
		watchLog, watchUser, err := scanWatchLogWithUser(rows)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}

		logs = append(logs, models.WatchLogResponse{
			WatchLog: *watchLog,
			User:     *watchUser,
		})
	}
	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to iterate post watch logs: %w", err)
	}

	response := &models.PostWatchLogsResponse{
		WatchCount: watchCount,
		Logs:       logs,
	}
	if avgRating.Valid {
		avg := avgRating.Float64
		response.AvgRating = &avg
	}

	if viewerID != nil {
		viewerLog, err := s.getViewerWatchLog(ctx, postID, *viewerID)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		if viewerLog != nil {
			response.ViewerWatched = true
			response.ViewerRating = &viewerLog.Rating
		}
	}

	return response, nil
}

func validateWatchLogRating(rating int) error {
	if rating < 1 || rating > 5 {
		return errors.New("rating must be between 1 and 5")
	}
	return nil
}

func (s *WatchLogService) verifyWatchablePost(ctx context.Context, postID uuid.UUID) error {
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
		return fmt.Errorf("failed to verify watchable post: %w", err)
	}
	if sectionType != "movie" && sectionType != "series" {
		return errors.New("post is not a movie or series")
	}
	return nil
}

func (s *WatchLogService) getExistingWatchLog(ctx context.Context, userID, postID uuid.UUID) (*models.WatchLog, error) {
	query := `
		SELECT id, user_id, post_id, rating, notes, watched_at, created_at, updated_at, deleted_at
		FROM watch_logs
		WHERE user_id = $1 AND post_id = $2
		ORDER BY deleted_at NULLS FIRST, watched_at DESC
		LIMIT 1
	`

	var log models.WatchLog
	var notes sql.NullString
	var updatedAt sql.NullTime
	var deletedAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, query, userID, postID).Scan(
		&log.ID, &log.UserID, &log.PostID, &log.Rating, &notes, &log.WatchedAt, &log.CreatedAt, &updatedAt, &deletedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to check existing watch log: %w", err)
	}

	if notes.Valid {
		log.Notes = &notes.String
	}
	if updatedAt.Valid {
		log.UpdatedAt = &updatedAt.Time
	}
	if deletedAt.Valid {
		log.DeletedAt = &deletedAt.Time
	}

	return &log, nil
}

func (s *WatchLogService) createWatchLog(ctx context.Context, userID, postID uuid.UUID, rating int, notes string) (*models.WatchLog, error) {
	query := `
		INSERT INTO watch_logs (id, user_id, post_id, rating, notes, watched_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		RETURNING id, user_id, post_id, rating, notes, watched_at, created_at, updated_at, deleted_at
	`

	watchLogID := uuid.New()
	watchedAt := s.now().UTC()
	var log models.WatchLog
	var note sql.NullString
	var updatedAt sql.NullTime
	var deletedAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, query, watchLogID, userID, postID, rating, normalizeWatchLogNote(notes), watchedAt).Scan(
		&log.ID, &log.UserID, &log.PostID, &log.Rating, &note, &log.WatchedAt, &log.CreatedAt, &updatedAt, &deletedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to create watch log: %w", err)
	}

	if note.Valid {
		log.Notes = &note.String
	}
	if updatedAt.Valid {
		log.UpdatedAt = &updatedAt.Time
	}
	if deletedAt.Valid {
		log.DeletedAt = &deletedAt.Time
	}

	return &log, nil
}

func (s *WatchLogService) restoreWatchLog(ctx context.Context, watchLogID uuid.UUID, rating int, notes string) (*models.WatchLog, error) {
	query := `
		UPDATE watch_logs
		SET deleted_at = NULL,
			rating = $2,
			notes = $3,
			watched_at = $4,
			updated_at = now()
		WHERE id = $1
		RETURNING id, user_id, post_id, rating, notes, watched_at, created_at, updated_at, deleted_at
	`

	watchedAt := s.now().UTC()
	var log models.WatchLog
	var note sql.NullString
	var updatedAt sql.NullTime
	var deletedAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, query, watchLogID, rating, normalizeWatchLogNote(notes), watchedAt).Scan(
		&log.ID, &log.UserID, &log.PostID, &log.Rating, &note, &log.WatchedAt, &log.CreatedAt, &updatedAt, &deletedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to restore watch log: %w", err)
	}

	if note.Valid {
		log.Notes = &note.String
	}
	if updatedAt.Valid {
		log.UpdatedAt = &updatedAt.Time
	}
	if deletedAt.Valid {
		log.DeletedAt = &deletedAt.Time
	}

	return &log, nil
}

func (s *WatchLogService) updateWatchLog(ctx context.Context, watchLogID uuid.UUID, rating *int, notes *string) (*models.WatchLog, error) {
	setClauses := make([]string, 0, 3)
	args := make([]interface{}, 0, 4)
	args = append(args, watchLogID)
	argIndex := 2

	if rating != nil {
		setClauses = append(setClauses, fmt.Sprintf("rating = $%d", argIndex))
		args = append(args, *rating)
		argIndex++
	}
	if notes != nil {
		setClauses = append(setClauses, fmt.Sprintf("notes = $%d", argIndex))
		args = append(args, normalizeWatchLogNote(*notes))
	}
	setClauses = append(setClauses, "updated_at = now()")

	query := fmt.Sprintf(`
		UPDATE watch_logs
		SET %s
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, user_id, post_id, rating, notes, watched_at, created_at, updated_at, deleted_at
	`, strings.Join(setClauses, ", "))

	var log models.WatchLog
	var note sql.NullString
	var updatedAt sql.NullTime
	var deletedAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&log.ID, &log.UserID, &log.PostID, &log.Rating, &note, &log.WatchedAt, &log.CreatedAt, &updatedAt, &deletedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("watch log not found")
		}
		return nil, fmt.Errorf("failed to update watch log: %w", err)
	}

	if note.Valid {
		log.Notes = &note.String
	}
	if updatedAt.Valid {
		log.UpdatedAt = &updatedAt.Time
	}
	if deletedAt.Valid {
		log.DeletedAt = &deletedAt.Time
	}

	return &log, nil
}

func (s *WatchLogService) getViewerWatchLog(ctx context.Context, postID, viewerID uuid.UUID) (*models.WatchLog, error) {
	query := `
		SELECT id, user_id, post_id, rating, notes, watched_at, created_at, updated_at, deleted_at
		FROM watch_logs
		WHERE post_id = $1 AND user_id = $2 AND deleted_at IS NULL
	`

	var log models.WatchLog
	var notes sql.NullString
	var updatedAt sql.NullTime
	var deletedAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, query, postID, viewerID).Scan(
		&log.ID, &log.UserID, &log.PostID, &log.Rating, &notes, &log.WatchedAt, &log.CreatedAt, &updatedAt, &deletedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch viewer watch log: %w", err)
	}

	if notes.Valid {
		log.Notes = &notes.String
	}
	if updatedAt.Valid {
		log.UpdatedAt = &updatedAt.Time
	}
	if deletedAt.Valid {
		log.DeletedAt = &deletedAt.Time
	}

	return &log, nil
}

func (s *WatchLogService) logWatchAudit(ctx context.Context, action string, userID uuid.UUID, metadata map[string]interface{}) error {
	if err := s.auditService.LogAuditWithMetadata(ctx, action, uuid.Nil, userID, metadata); err != nil {
		return fmt.Errorf("failed to create watch log audit log: %w", err)
	}
	return nil
}

func buildWatchLogCursor(watchedAt time.Time, watchLogID uuid.UUID) string {
	return watchedAt.UTC().Format(time.RFC3339Nano) + watchLogCursorSeparator + watchLogID.String()
}

func parseWatchLogCursor(cursor string) (time.Time, uuid.UUID, bool, error) {
	parts := strings.Split(cursor, watchLogCursorSeparator)
	if len(parts) == 2 {
		watchedAt, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return time.Time{}, uuid.Nil, false, errors.New("invalid cursor")
		}

		watchLogID, err := uuid.Parse(parts[1])
		if err != nil {
			return time.Time{}, uuid.Nil, false, errors.New("invalid cursor")
		}

		return watchedAt.UTC(), watchLogID, true, nil
	}

	// Backward compatibility for existing timestamp-only cursors.
	watchedAt, err := time.Parse(watchLogLegacyCursorLayout, cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, false, errors.New("invalid cursor")
	}
	return watchedAt.UTC(), uuid.Nil, false, nil
}

func normalizeWatchLogNote(note string) interface{} {
	trimmed := strings.TrimSpace(note)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func scanWatchLogWithPost(rows *sql.Rows) (*models.WatchLog, *models.Post, error) {
	var log models.WatchLog
	var logNotes sql.NullString
	var logUpdatedAt sql.NullTime
	var logDeletedAt sql.NullTime
	var post models.Post
	var postUpdatedAt sql.NullTime
	var postDeletedAt sql.NullTime
	var postDeletedBy sql.NullString

	if err := rows.Scan(
		&log.ID, &log.UserID, &log.PostID, &log.Rating, &logNotes, &log.WatchedAt, &log.CreatedAt, &logUpdatedAt, &logDeletedAt,
		&post.ID, &post.UserID, &post.SectionID, &post.Content, &post.CreatedAt, &postUpdatedAt, &postDeletedAt, &postDeletedBy,
	); err != nil {
		return nil, nil, fmt.Errorf("failed to scan watch log: %w", err)
	}

	if logNotes.Valid {
		log.Notes = &logNotes.String
	}
	if logUpdatedAt.Valid {
		log.UpdatedAt = &logUpdatedAt.Time
	}
	if logDeletedAt.Valid {
		log.DeletedAt = &logDeletedAt.Time
	}

	if postUpdatedAt.Valid {
		post.UpdatedAt = &postUpdatedAt.Time
	}
	if postDeletedAt.Valid {
		post.DeletedAt = &postDeletedAt.Time
	}
	if postDeletedBy.Valid {
		parsedID, err := uuid.Parse(postDeletedBy.String)
		if err == nil {
			post.DeletedByUserID = &parsedID
		}
	}

	return &log, &post, nil
}

func scanWatchLogWithUser(rows *sql.Rows) (*models.WatchLog, *models.WatchLogUser, error) {
	var log models.WatchLog
	var logNotes sql.NullString
	var logUpdatedAt sql.NullTime
	var logDeletedAt sql.NullTime
	var user models.WatchLogUser

	if err := rows.Scan(
		&log.ID, &log.UserID, &log.PostID, &log.Rating, &logNotes, &log.WatchedAt, &log.CreatedAt, &logUpdatedAt, &logDeletedAt,
		&user.ID, &user.Username, &user.ProfilePictureUrl,
	); err != nil {
		return nil, nil, fmt.Errorf("failed to scan post watch log: %w", err)
	}

	if logNotes.Valid {
		log.Notes = &logNotes.String
	}
	if logUpdatedAt.Valid {
		log.UpdatedAt = &logUpdatedAt.Time
	}
	if logDeletedAt.Valid {
		log.DeletedAt = &logDeletedAt.Time
	}

	return &log, &user, nil
}
