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
	defaultPodcastSaveListLimit = 20
	maxPodcastSaveListLimit     = 100
	podcastSaveCursorSeparator  = "|"
	podcastSaveLegacyCursor     = "2006-01-02T15:04:05.000Z07:00"
)

type podcastSavePostService interface {
	GetPostByID(ctx context.Context, postID uuid.UUID, userID uuid.UUID) (*models.Post, error)
}

// PodcastSaveService handles save-for-later operations for podcast posts.
type PodcastSaveService struct {
	db          *sql.DB
	postService podcastSavePostService
	audit       *AuditService
}

// NewPodcastSaveService creates a podcast save service with default dependencies.
func NewPodcastSaveService(db *sql.DB) *PodcastSaveService {
	return NewPodcastSaveServiceWithDependencies(db, NewPostService(db), NewAuditService(db))
}

// NewPodcastSaveServiceWithDependencies creates a podcast save service with explicit dependencies.
func NewPodcastSaveServiceWithDependencies(
	db *sql.DB,
	postService podcastSavePostService,
	auditService *AuditService,
) *PodcastSaveService {
	if postService == nil {
		postService = NewPostService(db)
	}
	if auditService == nil {
		auditService = NewAuditService(db)
	}

	return &PodcastSaveService{
		db:          db,
		postService: postService,
		audit:       auditService,
	}
}

// SavePodcast saves or restores a podcast post for a user.
func (s *PodcastSaveService) SavePodcast(ctx context.Context, userID, postID uuid.UUID) (*models.PodcastSave, error) {
	ctx, span := otel.Tracer("clubhouse.podcast_saves").Start(ctx, "PodcastSaveService.SavePodcast")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
	)
	defer span.End()

	if err := s.verifyPodcastPost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	existing, err := s.getMostRecentPodcastSave(ctx, userID, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if existing != nil {
		if existing.DeletedAt == nil {
			return existing, nil
		}

		restored, err := s.restorePodcastSave(ctx, existing.ID)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}

		if err := s.logPodcastSaveAudit(ctx, "save_podcast", userID, map[string]interface{}{
			"post_id": postID.String(),
		}); err != nil {
			recordSpanError(span, err)
			return nil, err
		}

		return restored, nil
	}

	created, err := s.createPodcastSave(ctx, userID, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if err := s.logPodcastSaveAudit(ctx, "save_podcast", userID, map[string]interface{}{
		"post_id": postID.String(),
	}); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return created, nil
}

// UnsavePodcast soft-deletes an active podcast save. It is idempotent.
func (s *PodcastSaveService) UnsavePodcast(ctx context.Context, userID, postID uuid.UUID) error {
	ctx, span := otel.Tracer("clubhouse.podcast_saves").Start(ctx, "PodcastSaveService.UnsavePodcast")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
	)
	defer span.End()

	if err := s.verifyPodcastPost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE podcast_saves
		SET deleted_at = now()
		WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL
	`, userID, postID)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to unsave podcast: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	if rowsAffected == 0 {
		return nil
	}

	if err := s.logPodcastSaveAudit(ctx, "unsave_podcast", userID, map[string]interface{}{
		"post_id": postID.String(),
	}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

// GetPostPodcastSaveInfo returns aggregate and viewer-specific save information for a podcast post.
func (s *PodcastSaveService) GetPostPodcastSaveInfo(ctx context.Context, postID uuid.UUID, viewerID *uuid.UUID) (*models.PostPodcastSaveInfo, error) {
	ctx, span := otel.Tracer("clubhouse.podcast_saves").Start(ctx, "PodcastSaveService.GetPostPodcastSaveInfo")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.Bool("has_viewer_id", viewerID != nil),
	)
	if viewerID != nil {
		span.SetAttributes(attribute.String("viewer_id", viewerID.String()))
	}
	defer span.End()

	if err := s.verifyPodcastPost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	var saveCount int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT user_id)
		FROM podcast_saves
		WHERE post_id = $1 AND deleted_at IS NULL
	`, postID).Scan(&saveCount); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to query podcast save count: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.username, u.profile_picture_url, MIN(ps.created_at) AS first_saved
		FROM podcast_saves ps
		JOIN users u ON ps.user_id = u.id
		WHERE ps.post_id = $1 AND ps.deleted_at IS NULL
		GROUP BY u.id, u.username, u.profile_picture_url
		ORDER BY first_saved ASC
	`, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to query podcast save users: %w", err)
	}
	defer rows.Close()

	users := make([]models.ReactionUser, 0)
	for rows.Next() {
		var user models.ReactionUser
		var firstSaved sql.NullTime
		if err := rows.Scan(&user.ID, &user.Username, &user.ProfilePictureUrl, &firstSaved); err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to iterate podcast save users: %w", err)
	}

	info := &models.PostPodcastSaveInfo{
		SaveCount: saveCount,
		Users:     users,
	}

	if viewerID != nil {
		var viewerSaved bool
		if err := s.db.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1
				FROM podcast_saves
				WHERE post_id = $1 AND user_id = $2 AND deleted_at IS NULL
			)
		`, postID, *viewerID).Scan(&viewerSaved); err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to query viewer podcast save state: %w", err)
		}
		info.ViewerSaved = viewerSaved
	}

	return info, nil
}

// ListSectionSavedPodcastPosts lists the viewer's saved podcast posts for one podcast section.
func (s *PodcastSaveService) ListSectionSavedPodcastPosts(
	ctx context.Context,
	sectionID, viewerID uuid.UUID,
	cursor *string,
	limit int,
) (*models.FeedResponse, error) {
	ctx, span := otel.Tracer("clubhouse.podcast_saves").Start(ctx, "PodcastSaveService.ListSectionSavedPodcastPosts")
	span.SetAttributes(
		attribute.String("section_id", sectionID.String()),
		attribute.String("viewer_id", viewerID.String()),
		attribute.Int("limit", limit),
		attribute.Bool("has_cursor", cursor != nil && strings.TrimSpace(*cursor) != ""),
	)
	defer span.End()

	if err := s.verifyPodcastSection(ctx, sectionID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if limit <= 0 || limit > maxPodcastSaveListLimit {
		limit = defaultPodcastSaveListLimit
	}

	query := `
		SELECT ps.id, ps.post_id, ps.created_at
		FROM podcast_saves ps
		JOIN posts p ON p.id = ps.post_id AND p.deleted_at IS NULL
		WHERE ps.user_id = $1 AND ps.deleted_at IS NULL AND p.section_id = $2
	`
	args := []interface{}{viewerID, sectionID}
	argIndex := 3

	if cursor != nil && strings.TrimSpace(*cursor) != "" {
		cursorCreatedAt, cursorID, hasID, err := parsePodcastSaveCursor(strings.TrimSpace(*cursor))
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		if hasID {
			query += fmt.Sprintf(" AND (ps.created_at < $%d OR (ps.created_at = $%d AND ps.id < $%d))", argIndex, argIndex, argIndex+1)
			args = append(args, cursorCreatedAt, cursorID)
			argIndex += 2
		} else {
			query += fmt.Sprintf(" AND ps.created_at < $%d", argIndex)
			args = append(args, cursorCreatedAt)
			argIndex++
		}
	}

	query += fmt.Sprintf(" ORDER BY ps.created_at DESC, ps.id DESC LIMIT $%d", argIndex)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to query podcast saves: %w", err)
	}
	defer rows.Close()

	type saveCursorRow struct {
		ID        uuid.UUID
		PostID    uuid.UUID
		CreatedAt time.Time
	}

	saveRows := make([]saveCursorRow, 0, limit+1)
	for rows.Next() {
		var row saveCursorRow
		if err := rows.Scan(&row.ID, &row.PostID, &row.CreatedAt); err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		saveRows = append(saveRows, row)
	}
	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to iterate podcast saves: %w", err)
	}

	hasMore := len(saveRows) > limit
	if hasMore {
		saveRows = saveRows[:limit]
	}

	posts := make([]*models.Post, 0, len(saveRows))
	for _, saveRow := range saveRows {
		post, err := s.postService.GetPostByID(ctx, saveRow.PostID, viewerID)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		posts = append(posts, post)
	}

	var nextCursor *string
	if hasMore && len(saveRows) > 0 {
		cursorValue := buildPodcastSaveCursor(saveRows[len(saveRows)-1].CreatedAt, saveRows[len(saveRows)-1].ID)
		nextCursor = &cursorValue
	}

	return &models.FeedResponse{
		Posts:      posts,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

func (s *PodcastSaveService) verifyPodcastSection(ctx context.Context, sectionID uuid.UUID) error {
	var sectionType string
	if err := s.db.QueryRowContext(ctx, "SELECT type FROM sections WHERE id = $1", sectionID).Scan(&sectionType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("section not found")
		}
		return fmt.Errorf("failed to verify section: %w", err)
	}

	if sectionType != "podcast" {
		return errors.New("section is not podcast")
	}

	return nil
}

func (s *PodcastSaveService) verifyPodcastPost(ctx context.Context, postID uuid.UUID) error {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM posts p
			JOIN sections s ON p.section_id = s.id
			WHERE p.id = $1 AND p.deleted_at IS NULL AND s.type = 'podcast'
		)
	`, postID).Scan(&exists); err != nil {
		return fmt.Errorf("failed to verify podcast post: %w", err)
	}
	if !exists {
		return errors.New("podcast post not found")
	}
	return nil
}

func (s *PodcastSaveService) getMostRecentPodcastSave(ctx context.Context, userID, postID uuid.UUID) (*models.PodcastSave, error) {
	var save models.PodcastSave
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, post_id, created_at, deleted_at
		FROM podcast_saves
		WHERE user_id = $1 AND post_id = $2
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, userID, postID).Scan(&save.ID, &save.UserID, &save.PostID, &save.CreatedAt, &save.DeletedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load podcast save: %w", err)
	}
	return &save, nil
}

func (s *PodcastSaveService) restorePodcastSave(ctx context.Context, saveID uuid.UUID) (*models.PodcastSave, error) {
	var save models.PodcastSave
	if err := s.db.QueryRowContext(ctx, `
		UPDATE podcast_saves
		SET deleted_at = NULL
		WHERE id = $1
		RETURNING id, user_id, post_id, created_at, deleted_at
	`, saveID).Scan(&save.ID, &save.UserID, &save.PostID, &save.CreatedAt, &save.DeletedAt); err != nil {
		return nil, fmt.Errorf("failed to restore podcast save: %w", err)
	}
	return &save, nil
}

func (s *PodcastSaveService) createPodcastSave(ctx context.Context, userID, postID uuid.UUID) (*models.PodcastSave, error) {
	var save models.PodcastSave
	if err := s.db.QueryRowContext(ctx, `
		INSERT INTO podcast_saves (id, user_id, post_id, created_at)
		VALUES ($1, $2, $3, now())
		RETURNING id, user_id, post_id, created_at, deleted_at
	`, uuid.New(), userID, postID).Scan(&save.ID, &save.UserID, &save.PostID, &save.CreatedAt, &save.DeletedAt); err != nil {
		return nil, fmt.Errorf("failed to create podcast save: %w", err)
	}
	return &save, nil
}

func (s *PodcastSaveService) logPodcastSaveAudit(ctx context.Context, action string, userID uuid.UUID, metadata map[string]interface{}) error {
	if err := s.audit.LogAuditWithMetadata(ctx, action, uuid.Nil, userID, metadata); err != nil {
		return fmt.Errorf("failed to create podcast save audit log: %w", err)
	}
	return nil
}

func parsePodcastSaveCursor(cursor string) (time.Time, uuid.UUID, bool, error) {
	parts := strings.Split(cursor, podcastSaveCursorSeparator)
	switch len(parts) {
	case 1:
		createdAt, err := parsePodcastSaveCursorTime(parts[0])
		if err != nil {
			return time.Time{}, uuid.Nil, false, errors.New("invalid cursor")
		}
		return createdAt, uuid.Nil, false, nil
	case 2:
		createdAt, err := parsePodcastSaveCursorTime(parts[0])
		if err != nil {
			return time.Time{}, uuid.Nil, false, errors.New("invalid cursor")
		}
		cursorID, err := uuid.Parse(parts[1])
		if err != nil {
			return time.Time{}, uuid.Nil, false, errors.New("invalid cursor")
		}
		return createdAt, cursorID, true, nil
	default:
		return time.Time{}, uuid.Nil, false, errors.New("invalid cursor")
	}
}

func parsePodcastSaveCursorTime(raw string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err == nil {
		return parsed, nil
	}
	return time.Parse(podcastSaveLegacyCursor, raw)
}

func buildPodcastSaveCursor(createdAt time.Time, saveID uuid.UUID) string {
	return fmt.Sprintf("%s%s%s", createdAt.UTC().Format(time.RFC3339Nano), podcastSaveCursorSeparator, saveID.String())
}
