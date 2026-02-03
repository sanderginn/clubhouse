package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// CookLogService handles cook log operations.
type CookLogService struct {
	db *sql.DB
}

// NewCookLogService creates a new cook log service.
func NewCookLogService(db *sql.DB) *CookLogService {
	return &CookLogService{db: db}
}

// LogCook creates or restores a cook log for a recipe post.
func (s *CookLogService) LogCook(ctx context.Context, userID, postID uuid.UUID, rating int, notes *string) (*models.CookLog, error) {
	ctx, span := otel.Tracer("clubhouse.cook_logs").Start(ctx, "CookLogService.LogCook")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
		attribute.Int("rating", rating),
		attribute.Bool("has_notes", notes != nil && strings.TrimSpace(*notes) != ""),
	)
	defer span.End()

	if err := validateCookLogRating(rating); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if err := s.verifyRecipePost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	existing, err := s.getExistingCookLog(ctx, userID, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if existing != nil {
		if existing.DeletedAt != nil {
			cookLog, err := s.restoreCookLog(ctx, existing.ID, rating, notes)
			if err != nil {
				recordSpanError(span, err)
				return nil, err
			}
			if err := s.logCookAudit(ctx, "log_cook", userID, map[string]interface{}{
				"post_id": postID.String(),
				"rating":  rating,
			}); err != nil {
				recordSpanError(span, err)
				return nil, err
			}
			return cookLog, nil
		}
		return existing, nil
	}

	cookLog, err := s.createCookLog(ctx, userID, postID, rating, notes)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if err := s.logCookAudit(ctx, "log_cook", userID, map[string]interface{}{
		"post_id": postID.String(),
		"rating":  rating,
	}); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return cookLog, nil
}

// UpdateCookLog updates an existing cook log for a recipe post.
func (s *CookLogService) UpdateCookLog(ctx context.Context, userID, postID uuid.UUID, rating int, notes *string) (*models.CookLog, error) {
	ctx, span := otel.Tracer("clubhouse.cook_logs").Start(ctx, "CookLogService.UpdateCookLog")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
		attribute.Int("rating", rating),
		attribute.Bool("has_notes", notes != nil && strings.TrimSpace(*notes) != ""),
	)
	defer span.End()

	if err := validateCookLogRating(rating); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if err := s.verifyRecipePost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	existing, err := s.getExistingCookLog(ctx, userID, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	if existing == nil || existing.DeletedAt != nil {
		notFoundErr := errors.New("cook log not found")
		recordSpanError(span, notFoundErr)
		return nil, notFoundErr
	}

	cookLog, err := s.updateCookLog(ctx, existing.ID, rating, notes)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if err := s.logCookAudit(ctx, "update_cook_log", userID, map[string]interface{}{
		"post_id":    postID.String(),
		"old_rating": existing.Rating,
		"new_rating": rating,
	}); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return cookLog, nil
}

// RemoveCookLog deletes a cook log for a recipe post.
func (s *CookLogService) RemoveCookLog(ctx context.Context, userID, postID uuid.UUID) error {
	ctx, span := otel.Tracer("clubhouse.cook_logs").Start(ctx, "CookLogService.RemoveCookLog")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
	)
	defer span.End()

	if err := s.verifyRecipePost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return err
	}

	query := `
		UPDATE cook_logs
		SET deleted_at = now(), updated_at = now()
		WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL
	`

	result, err := s.db.ExecContext(ctx, query, userID, postID)
	if err != nil {
		recordSpanError(span, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		recordSpanError(span, err)
		return err
	}

	if rowsAffected == 0 {
		notFoundErr := errors.New("cook log not found")
		recordSpanError(span, notFoundErr)
		return notFoundErr
	}

	if err := s.logCookAudit(ctx, "delete_cook_log", userID, map[string]interface{}{
		"post_id": postID.String(),
	}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

// GetPostCookLogs retrieves cook log info for a post.
func (s *CookLogService) GetPostCookLogs(ctx context.Context, postID uuid.UUID, viewerID *uuid.UUID) (*models.PostCookInfo, error) {
	ctx, span := otel.Tracer("clubhouse.cook_logs").Start(ctx, "CookLogService.GetPostCookLogs")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.Bool("has_viewer", viewerID != nil),
	)
	defer span.End()

	if err := s.verifyRecipePost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	var cookCount int
	var avgRating sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), AVG(rating)
		FROM cook_logs
		WHERE post_id = $1 AND deleted_at IS NULL
	`, postID).Scan(&cookCount, &avgRating); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to fetch cook log summary: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.username, u.profile_picture_url, cl.rating, cl.created_at
		FROM cook_logs cl
		JOIN users u ON cl.user_id = u.id
		WHERE cl.post_id = $1 AND cl.deleted_at IS NULL
		ORDER BY cl.created_at DESC
	`, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to query cook log users: %w", err)
	}
	defer rows.Close()

	users := []models.CookLogUser{}
	for rows.Next() {
		var user models.CookLogUser
		if err := rows.Scan(&user.ID, &user.Username, &user.ProfilePictureUrl, &user.Rating, &user.CreatedAt); err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to scan cook log user: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to iterate cook log users: %w", err)
	}

	info := &models.PostCookInfo{
		CookCount: cookCount,
		Users:     users,
	}

	if avgRating.Valid {
		avg := avgRating.Float64
		info.AvgRating = &avg
	}

	if viewerID != nil {
		viewerLog, err := s.getViewerCookLog(ctx, postID, *viewerID)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		if viewerLog != nil {
			info.ViewerCooked = true
			info.ViewerCookLog = viewerLog
		}
	}

	return info, nil
}

// GetUserCookLogs retrieves cook logs for a user.
func (s *CookLogService) GetUserCookLogs(ctx context.Context, userID uuid.UUID, limit int, cursor *string) ([]models.CookLogWithPost, bool, *string, error) {
	ctx, span := otel.Tracer("clubhouse.cook_logs").Start(ctx, "CookLogService.GetUserCookLogs")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.Bool("has_cursor", cursor != nil && *cursor != ""),
		attribute.Int("limit", limit),
	)
	defer span.End()

	var exists bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL AND approved_at IS NOT NULL)
	`, userID).Scan(&exists); err != nil {
		recordSpanError(span, err)
		return nil, false, nil, fmt.Errorf("failed to check user: %w", err)
	}
	if !exists {
		notFoundErr := errors.New("user not found")
		recordSpanError(span, notFoundErr)
		return nil, false, nil, notFoundErr
	}

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	query := `
		SELECT
			cl.id, cl.user_id, cl.post_id, cl.rating, cl.notes, cl.created_at, cl.updated_at, cl.deleted_at,
			p.id, p.user_id, p.section_id, p.content, p.created_at, p.updated_at, p.deleted_at, p.deleted_by_user_id
		FROM cook_logs cl
		JOIN posts p ON cl.post_id = p.id AND p.deleted_at IS NULL
		WHERE cl.user_id = $1 AND cl.deleted_at IS NULL
	`

	args := []interface{}{userID}
	argIndex := 2

	if cursor != nil && *cursor != "" {
		query += fmt.Sprintf(" AND cl.created_at < $%d", argIndex)
		args = append(args, *cursor)
		argIndex++
	}

	query += fmt.Sprintf(" ORDER BY cl.created_at DESC LIMIT $%d", argIndex)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		recordSpanError(span, err)
		return nil, false, nil, fmt.Errorf("failed to query cook logs: %w", err)
	}
	defer rows.Close()

	logs := []models.CookLogWithPost{}
	for rows.Next() {
		var log models.CookLog
		var notes sql.NullString
		var logUpdatedAt sql.NullTime
		var logDeletedAt sql.NullTime
		var post models.Post
		var postUpdatedAt sql.NullTime
		var postDeletedAt sql.NullTime
		var postDeletedBy sql.NullString

		if err := rows.Scan(
			&log.ID, &log.UserID, &log.PostID, &log.Rating, &notes, &log.CreatedAt, &logUpdatedAt, &logDeletedAt,
			&post.ID, &post.UserID, &post.SectionID, &post.Content, &post.CreatedAt, &postUpdatedAt, &postDeletedAt, &postDeletedBy,
		); err != nil {
			recordSpanError(span, err)
			return nil, false, nil, fmt.Errorf("failed to scan cook log: %w", err)
		}

		if notes.Valid {
			log.Notes = &notes.String
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
			parsedID, _ := uuid.Parse(postDeletedBy.String)
			post.DeletedByUserID = &parsedID
		}

		logs = append(logs, models.CookLogWithPost{
			CookLog: log,
			Post:    &post,
		})
	}

	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, false, nil, fmt.Errorf("failed to iterate cook logs: %w", err)
	}

	hasMore := len(logs) > limit
	if hasMore {
		logs = logs[:limit]
	}

	var nextCursor *string
	if hasMore && len(logs) > 0 {
		cursorStr := logs[len(logs)-1].CreatedAt.Format("2006-01-02T15:04:05.000Z07:00")
		nextCursor = &cursorStr
	}

	return logs, hasMore, nextCursor, nil
}

func validateCookLogRating(rating int) error {
	if rating < 1 || rating > 5 {
		return fmt.Errorf("rating must be between 1 and 5")
	}
	return nil
}

func (s *CookLogService) verifyRecipePost(ctx context.Context, postID uuid.UUID) error {
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
		return fmt.Errorf("failed to verify recipe post: %w", err)
	}
	if sectionType != "recipe" {
		return errors.New("post is not a recipe")
	}
	return nil
}

func (s *CookLogService) getExistingCookLog(ctx context.Context, userID, postID uuid.UUID) (*models.CookLog, error) {
	query := `
		SELECT id, user_id, post_id, rating, notes, created_at, updated_at, deleted_at
		FROM cook_logs
		WHERE user_id = $1 AND post_id = $2
	`

	var log models.CookLog
	var notes sql.NullString
	var updatedAt sql.NullTime
	var deletedAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, query, userID, postID).Scan(
		&log.ID, &log.UserID, &log.PostID, &log.Rating, &notes, &log.CreatedAt, &updatedAt, &deletedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to check existing cook log: %w", err)
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

func (s *CookLogService) createCookLog(ctx context.Context, userID, postID uuid.UUID, rating int, notes *string) (*models.CookLog, error) {
	query := `
		INSERT INTO cook_logs (id, user_id, post_id, rating, notes, created_at)
		VALUES ($1, $2, $3, $4, $5, now())
		RETURNING id, user_id, post_id, rating, notes, created_at, updated_at, deleted_at
	`

	logID := uuid.New()
	var log models.CookLog
	var noteValue interface{}
	if notes != nil {
		noteValue = strings.TrimSpace(*notes)
	}
	var note sql.NullString
	var updatedAt sql.NullTime
	var deletedAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, query, logID, userID, postID, rating, noteValue).Scan(
		&log.ID, &log.UserID, &log.PostID, &log.Rating, &note, &log.CreatedAt, &updatedAt, &deletedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to create cook log: %w", err)
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

func (s *CookLogService) restoreCookLog(ctx context.Context, logID uuid.UUID, rating int, notes *string) (*models.CookLog, error) {
	query := `
		UPDATE cook_logs
		SET deleted_at = NULL, rating = $2, notes = $3, updated_at = now()
		WHERE id = $1
		RETURNING id, user_id, post_id, rating, notes, created_at, updated_at, deleted_at
	`

	var log models.CookLog
	var noteValue interface{}
	if notes != nil {
		noteValue = strings.TrimSpace(*notes)
	}
	var note sql.NullString
	var updatedAt sql.NullTime
	var deletedAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, query, logID, rating, noteValue).Scan(
		&log.ID, &log.UserID, &log.PostID, &log.Rating, &note, &log.CreatedAt, &updatedAt, &deletedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to restore cook log: %w", err)
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

func (s *CookLogService) updateCookLog(ctx context.Context, logID uuid.UUID, rating int, notes *string) (*models.CookLog, error) {
	query := `
		UPDATE cook_logs
		SET rating = $2, notes = $3, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, user_id, post_id, rating, notes, created_at, updated_at, deleted_at
	`

	var log models.CookLog
	var noteValue interface{}
	if notes != nil {
		noteValue = strings.TrimSpace(*notes)
	}
	var note sql.NullString
	var updatedAt sql.NullTime
	var deletedAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, query, logID, rating, noteValue).Scan(
		&log.ID, &log.UserID, &log.PostID, &log.Rating, &note, &log.CreatedAt, &updatedAt, &deletedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to update cook log: %w", err)
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

func (s *CookLogService) getViewerCookLog(ctx context.Context, postID, viewerID uuid.UUID) (*models.CookLog, error) {
	query := `
		SELECT id, user_id, post_id, rating, notes, created_at, updated_at, deleted_at
		FROM cook_logs
		WHERE post_id = $1 AND user_id = $2 AND deleted_at IS NULL
	`

	var log models.CookLog
	var notes sql.NullString
	var updatedAt sql.NullTime
	var deletedAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, query, postID, viewerID).Scan(
		&log.ID, &log.UserID, &log.PostID, &log.Rating, &notes, &log.CreatedAt, &updatedAt, &deletedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch viewer cook log: %w", err)
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

func (s *CookLogService) logCookAudit(ctx context.Context, action string, userID uuid.UUID, metadata map[string]interface{}) error {
	auditService := NewAuditService(s.db)
	if err := auditService.LogAuditWithMetadata(ctx, action, uuid.Nil, userID, metadata); err != nil {
		return fmt.Errorf("failed to create cook log audit log: %w", err)
	}
	return nil
}
