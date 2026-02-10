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
	defaultBookshelfCategoryName  = "Uncategorized"
	maxBookshelfCategoryNameLimit = 100
	defaultBookshelfPageSize      = 20
	maxBookshelfPageSize          = 100
	bookshelfCursorSeparator      = "|"
)

// BookshelfService handles book bookshelf operations.
type BookshelfService struct {
	db    *sql.DB
	audit *AuditService
}

// NewBookshelfService creates a bookshelf service.
func NewBookshelfService(db *sql.DB) *BookshelfService {
	return &BookshelfService{
		db:    db,
		audit: NewAuditService(db),
	}
}

// CreateCategory creates a new bookshelf category for a user.
func (s *BookshelfService) CreateCategory(ctx context.Context, userID uuid.UUID, name string) (*models.BookshelfCategory, error) {
	ctx, span := otel.Tracer("clubhouse.bookshelf").Start(ctx, "BookshelfService.CreateCategory")
	span.SetAttributes(attribute.String("user_id", userID.String()))
	defer span.End()

	normalized, err := normalizeBookshelfCategoryName(name)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	span.SetAttributes(attribute.String("category_name", normalized))

	position, err := s.nextBookshelfCategoryPosition(ctx, s.db, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	query := `
		INSERT INTO bookshelf_categories (id, user_id, name, position, created_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING id, user_id, name, position, created_at
	`

	categoryID := uuid.New()
	var category models.BookshelfCategory
	if err := s.db.QueryRowContext(ctx, query, categoryID, userID, normalized, position).Scan(
		&category.ID,
		&category.UserID,
		&category.Name,
		&category.Position,
		&category.CreatedAt,
	); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to create bookshelf category: %w", err)
	}

	if err := s.logBookshelfAudit(ctx, "create_bookshelf_category", userID, map[string]interface{}{
		"category_id":   category.ID.String(),
		"category_name": category.Name,
	}); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return &category, nil
}

// GetCategories returns all bookshelf categories for a user in display order.
func (s *BookshelfService) GetCategories(ctx context.Context, userID uuid.UUID) ([]models.BookshelfCategory, error) {
	ctx, span := otel.Tracer("clubhouse.bookshelf").Start(ctx, "BookshelfService.GetCategories")
	span.SetAttributes(attribute.String("user_id", userID.String()))
	defer span.End()

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, name, position, created_at
		FROM bookshelf_categories
		WHERE user_id = $1
		ORDER BY position ASC, created_at ASC
	`, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to query bookshelf categories: %w", err)
	}
	defer rows.Close()

	categories := make([]models.BookshelfCategory, 0)
	for rows.Next() {
		var category models.BookshelfCategory
		if err := rows.Scan(&category.ID, &category.UserID, &category.Name, &category.Position, &category.CreatedAt); err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to iterate bookshelf categories: %w", err)
	}

	return categories, nil
}

// UpdateCategory updates a bookshelf category's name and position.
func (s *BookshelfService) UpdateCategory(
	ctx context.Context,
	userID, categoryID uuid.UUID,
	req models.UpdateBookshelfCategoryRequest,
) (*models.BookshelfCategory, error) {
	ctx, span := otel.Tracer("clubhouse.bookshelf").Start(ctx, "BookshelfService.UpdateCategory")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("category_id", categoryID.String()),
		attribute.Int("position", req.Position),
	)
	defer span.End()

	normalized, err := normalizeBookshelfCategoryName(req.Name)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	if req.Position < 0 {
		err := errors.New("position must be greater than or equal to 0")
		recordSpanError(span, err)
		return nil, err
	}

	query := `
		UPDATE bookshelf_categories
		SET name = $1, position = $2
		WHERE id = $3 AND user_id = $4
		RETURNING id, user_id, name, position, created_at
	`

	var category models.BookshelfCategory
	if err := s.db.QueryRowContext(ctx, query, normalized, req.Position, categoryID, userID).Scan(
		&category.ID,
		&category.UserID,
		&category.Name,
		&category.Position,
		&category.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := errors.New("category not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to update bookshelf category: %w", err)
	}

	if err := s.logBookshelfAudit(ctx, "update_bookshelf_category", userID, map[string]interface{}{
		"category_id": category.ID.String(),
		"changes": map[string]interface{}{
			"name":     category.Name,
			"position": category.Position,
		},
	}); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return &category, nil
}

// DeleteCategory deletes a category and moves its active items to uncategorized (NULL category_id).
func (s *BookshelfService) DeleteCategory(ctx context.Context, userID, categoryID uuid.UUID) error {
	ctx, span := otel.Tracer("clubhouse.bookshelf").Start(ctx, "BookshelfService.DeleteCategory")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("category_id", categoryID.String()),
	)
	defer span.End()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to begin delete bookshelf category transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var categoryName string
	if err := tx.QueryRowContext(
		ctx,
		"SELECT name FROM bookshelf_categories WHERE id = $1 AND user_id = $2",
		categoryID,
		userID,
	).Scan(&categoryName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := errors.New("category not found")
			recordSpanError(span, notFoundErr)
			return notFoundErr
		}
		recordSpanError(span, err)
		return fmt.Errorf("failed to load bookshelf category: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE bookshelf_items
		SET category_id = NULL
		WHERE user_id = $1 AND category_id = $2 AND deleted_at IS NULL`,
		userID,
		categoryID,
	); err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to move bookshelf items to uncategorized: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM bookshelf_categories WHERE id = $1 AND user_id = $2", categoryID, userID); err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to delete bookshelf category: %w", err)
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to commit delete bookshelf category transaction: %w", err)
	}

	if err := s.logBookshelfAudit(ctx, "delete_bookshelf_category", userID, map[string]interface{}{
		"category_id":   categoryID.String(),
		"category_name": categoryName,
	}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

// ReorderCategories updates category positions in the provided order.
func (s *BookshelfService) ReorderCategories(ctx context.Context, userID uuid.UUID, categoryIDs []uuid.UUID) error {
	ctx, span := otel.Tracer("clubhouse.bookshelf").Start(ctx, "BookshelfService.ReorderCategories")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.Int("category_count", len(categoryIDs)),
	)
	defer span.End()

	if len(categoryIDs) == 0 {
		err := errors.New("category_ids must not be empty")
		recordSpanError(span, err)
		return err
	}

	seen := make(map[uuid.UUID]struct{}, len(categoryIDs))
	for _, categoryID := range categoryIDs {
		if _, exists := seen[categoryID]; exists {
			err := errors.New("duplicate category id")
			recordSpanError(span, err)
			return err
		}
		seen[categoryID] = struct{}{}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to begin reorder bookshelf categories transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	rows, err := tx.QueryContext(ctx, "SELECT id FROM bookshelf_categories WHERE user_id = $1", userID)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to query user bookshelf categories: %w", err)
	}
	existing := make(map[uuid.UUID]struct{})
	for rows.Next() {
		var categoryID uuid.UUID
		if err := rows.Scan(&categoryID); err != nil {
			_ = rows.Close()
			recordSpanError(span, err)
			return err
		}
		existing[categoryID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		recordSpanError(span, err)
		return fmt.Errorf("failed to iterate user bookshelf categories: %w", err)
	}
	_ = rows.Close()

	if len(existing) != len(categoryIDs) {
		err := errors.New("category_ids must include all user categories")
		recordSpanError(span, err)
		return err
	}
	for _, categoryID := range categoryIDs {
		if _, ok := existing[categoryID]; !ok {
			err := errors.New("category not found")
			recordSpanError(span, err)
			return err
		}
	}

	for position, categoryID := range categoryIDs {
		if _, err := tx.ExecContext(
			ctx,
			"UPDATE bookshelf_categories SET position = $1 WHERE id = $2 AND user_id = $3",
			position,
			categoryID,
			userID,
		); err != nil {
			recordSpanError(span, err)
			return fmt.Errorf("failed to update category position: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to commit reorder bookshelf categories transaction: %w", err)
	}

	reordered := make([]string, 0, len(categoryIDs))
	for _, categoryID := range categoryIDs {
		reordered = append(reordered, categoryID.String())
	}
	if err := s.logBookshelfAudit(ctx, "update_bookshelf_category", userID, map[string]interface{}{
		"reordered_category_ids": reordered,
	}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

// AddToBookshelf adds or restores a bookshelf item.
func (s *BookshelfService) AddToBookshelf(ctx context.Context, userID, postID uuid.UUID, categories []string) error {
	ctx, span := otel.Tracer("clubhouse.bookshelf").Start(ctx, "BookshelfService.AddToBookshelf")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
		attribute.Int("category_count", len(categories)),
	)
	defer span.End()

	if err := s.verifyBookPost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return err
	}

	normalizedCategories, err := normalizeBookshelfCategoriesForAdd(categories)
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	span.SetAttributes(attribute.StringSlice("categories", normalizedCategories))

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to begin add bookshelf transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	selectedCategoryID, selectedCategoryName, autoCreatedCategories, err := s.resolveBookshelfCategories(ctx, tx, userID, normalizedCategories)
	if err != nil {
		recordSpanError(span, err)
		return err
	}

	changed, err := s.restoreDeletedBookshelfItem(ctx, tx, userID, postID, selectedCategoryID)
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	if !changed {
		changed, err = s.upsertActiveBookshelfItem(ctx, tx, userID, postID, selectedCategoryID)
		if err != nil {
			recordSpanError(span, err)
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to commit add bookshelf transaction: %w", err)
	}

	if changed {
		metadata := map[string]interface{}{
			"post_id":    postID.String(),
			"categories": normalizedCategories,
		}
		if selectedCategoryName != "" {
			metadata["selected_category"] = selectedCategoryName
		} else {
			metadata["selected_category"] = nil
		}
		if len(autoCreatedCategories) > 0 {
			metadata["auto_created_categories"] = autoCreatedCategories
		}

		if err := s.logBookshelfAudit(ctx, "add_to_bookshelf", userID, metadata); err != nil {
			recordSpanError(span, err)
			return err
		}
	}

	return nil
}

func (s *BookshelfService) restoreDeletedBookshelfItem(
	ctx context.Context,
	tx *sql.Tx,
	userID, postID uuid.UUID,
	categoryID *uuid.UUID,
) (bool, error) {
	query := `
		WITH candidate AS (
			SELECT id
			FROM bookshelf_items
			WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NOT NULL
			ORDER BY created_at DESC, id DESC
			LIMIT 1
			FOR UPDATE
		)
		UPDATE bookshelf_items bi
		SET category_id = $3, deleted_at = NULL
		FROM candidate
		WHERE bi.id = candidate.id
		RETURNING bi.id
	`

	var restoredID uuid.UUID
	err := tx.QueryRowContext(ctx, query, userID, postID, uuidPointerValue(categoryID)).Scan(&restoredID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to restore bookshelf item: %w", err)
	}

	return true, nil
}

func (s *BookshelfService) upsertActiveBookshelfItem(
	ctx context.Context,
	tx *sql.Tx,
	userID, postID uuid.UUID,
	categoryID *uuid.UUID,
) (bool, error) {
	query := `
		INSERT INTO bookshelf_items (id, user_id, post_id, category_id, created_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (user_id, post_id) WHERE deleted_at IS NULL
		DO UPDATE SET category_id = EXCLUDED.category_id
		WHERE bookshelf_items.category_id IS DISTINCT FROM EXCLUDED.category_id
		RETURNING id
	`

	var affectedID uuid.UUID
	err := tx.QueryRowContext(ctx, query, uuid.New(), userID, postID, uuidPointerValue(categoryID)).Scan(&affectedID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to create or update bookshelf item: %w", err)
	}

	return true, nil
}

// RemoveFromBookshelf soft deletes an active bookshelf item.
func (s *BookshelfService) RemoveFromBookshelf(ctx context.Context, userID, postID uuid.UUID) error {
	ctx, span := otel.Tracer("clubhouse.bookshelf").Start(ctx, "BookshelfService.RemoveFromBookshelf")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
	)
	defer span.End()

	if err := s.verifyBookPost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE bookshelf_items
		SET deleted_at = now()
		WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL
	`, userID, postID)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to remove bookshelf item: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	if rowsAffected == 0 {
		notFoundErr := errors.New("bookshelf item not found")
		recordSpanError(span, notFoundErr)
		return notFoundErr
	}

	if err := s.logBookshelfAudit(ctx, "remove_from_bookshelf", userID, map[string]interface{}{
		"post_id": postID.String(),
	}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

// GetUserBookshelf lists a user's bookshelf items with optional category filter and cursor pagination.
func (s *BookshelfService) GetUserBookshelf(
	ctx context.Context,
	userID uuid.UUID,
	category *string,
	cursor *string,
	limit int,
) ([]models.BookshelfItem, *string, error) {
	ctx, span := otel.Tracer("clubhouse.bookshelf").Start(ctx, "BookshelfService.GetUserBookshelf")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.Int("limit", limit),
		attribute.Bool("has_cursor", cursor != nil && strings.TrimSpace(*cursor) != ""),
		attribute.Bool("has_category", category != nil && strings.TrimSpace(*category) != ""),
	)
	defer span.End()

	items, nextCursor, err := s.getBookshelfItems(ctx, &userID, category, cursor, limit)
	if err != nil {
		recordSpanError(span, err)
		return nil, nil, err
	}
	return items, nextCursor, nil
}

// GetAllBookshelfItems lists all members' bookshelf items with optional category filter and cursor pagination.
func (s *BookshelfService) GetAllBookshelfItems(
	ctx context.Context,
	category *string,
	cursor *string,
	limit int,
) ([]models.BookshelfItem, *string, error) {
	ctx, span := otel.Tracer("clubhouse.bookshelf").Start(ctx, "BookshelfService.GetAllBookshelfItems")
	span.SetAttributes(
		attribute.Int("limit", limit),
		attribute.Bool("has_cursor", cursor != nil && strings.TrimSpace(*cursor) != ""),
		attribute.Bool("has_category", category != nil && strings.TrimSpace(*category) != ""),
	)
	defer span.End()

	items, nextCursor, err := s.getBookshelfItems(ctx, nil, category, cursor, limit)
	if err != nil {
		recordSpanError(span, err)
		return nil, nil, err
	}
	return items, nextCursor, nil
}

// GetPostBookshelfInfo returns save count, users, and viewer bookshelf state for one post.
func (s *BookshelfService) GetPostBookshelfInfo(ctx context.Context, postID uuid.UUID, viewerID *uuid.UUID) (*models.PostBookshelfInfo, error) {
	ctx, span := otel.Tracer("clubhouse.bookshelf").Start(ctx, "BookshelfService.GetPostBookshelfInfo")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.Bool("has_viewer", viewerID != nil),
	)
	if viewerID != nil {
		span.SetAttributes(attribute.String("viewer_id", viewerID.String()))
	}
	defer span.End()

	if err := s.verifyBookPost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	var saveCount int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT user_id)
		FROM bookshelf_items
		WHERE post_id = $1 AND deleted_at IS NULL
	`, postID).Scan(&saveCount); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to query bookshelf save count: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.username, u.profile_picture_url, MIN(bi.created_at) AS first_saved
		FROM bookshelf_items bi
		JOIN users u ON bi.user_id = u.id
		WHERE bi.post_id = $1 AND bi.deleted_at IS NULL
		GROUP BY u.id, u.username, u.profile_picture_url
		ORDER BY first_saved ASC
	`, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to query bookshelf users: %w", err)
	}
	defer rows.Close()

	users := make([]models.BookshelfUserInfo, 0)
	for rows.Next() {
		var user models.BookshelfUserInfo
		var firstSaved sql.NullTime
		if err := rows.Scan(&user.ID, &user.Username, &user.ProfilePictureUrl, &firstSaved); err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to iterate bookshelf users: %w", err)
	}

	info := &models.PostBookshelfInfo{
		SaveCount: saveCount,
		Users:     users,
	}

	if viewerID != nil {
		viewerRows, err := s.db.QueryContext(ctx, `
			SELECT bc.name
			FROM bookshelf_items bi
			LEFT JOIN bookshelf_categories bc ON bc.id = bi.category_id
			WHERE bi.post_id = $1 AND bi.user_id = $2 AND bi.deleted_at IS NULL
			ORDER BY bi.created_at ASC, bi.id ASC
		`, postID, *viewerID)
		if err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to query viewer bookshelf categories: %w", err)
		}
		defer viewerRows.Close()

		for viewerRows.Next() {
			var category sql.NullString
			if err := viewerRows.Scan(&category); err != nil {
				recordSpanError(span, err)
				return nil, err
			}
			info.ViewerCategories = append(info.ViewerCategories, bookshelfDisplayCategoryName(category))
		}
		if err := viewerRows.Err(); err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to iterate viewer bookshelf categories: %w", err)
		}
		if len(info.ViewerCategories) > 0 {
			info.ViewerSaved = true
		}
	}

	return info, nil
}

// GetBookshelfStatsForPosts returns bookshelf aggregation for multiple posts.
func (s *BookshelfService) GetBookshelfStatsForPosts(
	ctx context.Context,
	postIDs []uuid.UUID,
	viewerID *uuid.UUID,
) (map[uuid.UUID]*models.PostBookshelfInfo, error) {
	ctx, span := otel.Tracer("clubhouse.bookshelf").Start(ctx, "BookshelfService.GetBookshelfStatsForPosts")
	span.SetAttributes(
		attribute.Int("post_count", len(postIDs)),
		attribute.Bool("has_viewer", viewerID != nil),
	)
	if viewerID != nil {
		span.SetAttributes(attribute.String("viewer_id", viewerID.String()))
	}
	defer span.End()

	stats := make(map[uuid.UUID]*models.PostBookshelfInfo, len(postIDs))
	for _, postID := range postIDs {
		stats[postID] = &models.PostBookshelfInfo{}
	}
	if len(postIDs) == 0 {
		return stats, nil
	}

	viewerIDValue := uuid.Nil
	if viewerID != nil {
		viewerIDValue = *viewerID
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			bi.post_id,
			COUNT(DISTINCT bi.user_id) AS save_count,
			bool_or(bi.user_id = $2) AS viewer_saved
		FROM bookshelf_items bi
		WHERE bi.post_id = ANY($1) AND bi.deleted_at IS NULL
		GROUP BY bi.post_id
	`, pq.Array(postIDs), viewerIDValue)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to query bookshelf stats: %w", err)
	}
	for rows.Next() {
		var postID uuid.UUID
		var saveCount int
		var viewerSaved bool
		if err := rows.Scan(&postID, &saveCount, &viewerSaved); err != nil {
			_ = rows.Close()
			recordSpanError(span, err)
			return nil, err
		}
		if stat, ok := stats[postID]; ok {
			stat.SaveCount = saveCount
			stat.ViewerSaved = viewerSaved
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to iterate bookshelf stats: %w", err)
	}
	_ = rows.Close()

	if viewerID != nil {
		categoryRows, err := s.db.QueryContext(ctx, `
			SELECT bi.post_id, bc.name
			FROM bookshelf_items bi
			LEFT JOIN bookshelf_categories bc ON bc.id = bi.category_id
			WHERE bi.post_id = ANY($1) AND bi.user_id = $2 AND bi.deleted_at IS NULL
			ORDER BY bi.post_id, bi.created_at ASC, bi.id ASC
		`, pq.Array(postIDs), *viewerID)
		if err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to query viewer bookshelf categories for stats: %w", err)
		}
		for categoryRows.Next() {
			var postID uuid.UUID
			var category sql.NullString
			if err := categoryRows.Scan(&postID, &category); err != nil {
				_ = categoryRows.Close()
				recordSpanError(span, err)
				return nil, err
			}
			if stat, ok := stats[postID]; ok {
				stat.ViewerCategories = append(stat.ViewerCategories, bookshelfDisplayCategoryName(category))
			}
		}
		if err := categoryRows.Err(); err != nil {
			_ = categoryRows.Close()
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to iterate viewer bookshelf categories for stats: %w", err)
		}
		_ = categoryRows.Close()
	}

	return stats, nil
}

func (s *BookshelfService) getBookshelfItems(
	ctx context.Context,
	userID *uuid.UUID,
	category *string,
	cursor *string,
	limit int,
) ([]models.BookshelfItem, *string, error) {
	if limit <= 0 || limit > maxBookshelfPageSize {
		limit = defaultBookshelfPageSize
	}

	query := `
		SELECT bi.id, bi.user_id, bi.post_id, bi.category_id, bi.created_at, bi.deleted_at
		FROM bookshelf_items bi
		JOIN posts p ON p.id = bi.post_id AND p.deleted_at IS NULL
		LEFT JOIN bookshelf_categories bc ON bc.id = bi.category_id
		WHERE bi.deleted_at IS NULL
	`

	args := make([]interface{}, 0, 5)
	argIndex := 1

	if userID != nil {
		query += fmt.Sprintf(" AND bi.user_id = $%d", argIndex)
		args = append(args, *userID)
		argIndex++
	}

	if category != nil {
		trimmed := strings.TrimSpace(*category)
		if trimmed != "" {
			if strings.EqualFold(trimmed, defaultBookshelfCategoryName) {
				query += " AND bi.category_id IS NULL"
			} else {
				query += fmt.Sprintf(" AND bc.name = $%d", argIndex)
				args = append(args, trimmed)
				argIndex++
			}
		}
	}

	if cursor != nil && strings.TrimSpace(*cursor) != "" {
		cursorCreatedAt, cursorID, hasID, err := parseBookshelfCursor(strings.TrimSpace(*cursor))
		if err != nil {
			return nil, nil, err
		}
		if hasID {
			query += fmt.Sprintf(" AND (bi.created_at < $%d OR (bi.created_at = $%d AND bi.id < $%d))", argIndex, argIndex, argIndex+1)
			args = append(args, cursorCreatedAt, cursorID)
			argIndex += 2
		} else {
			query += fmt.Sprintf(" AND bi.created_at < $%d", argIndex)
			args = append(args, cursorCreatedAt)
			argIndex++
		}
	}

	query += fmt.Sprintf(" ORDER BY bi.created_at DESC, bi.id DESC LIMIT $%d", argIndex)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query bookshelf items: %w", err)
	}
	defer rows.Close()

	items := make([]models.BookshelfItem, 0, limit)
	for rows.Next() {
		item, err := scanBookshelfItem(rows)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("failed to iterate bookshelf items: %w", err)
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	var nextCursor *string
	if hasMore && len(items) > 0 {
		cursorValue := buildBookshelfCursor(items[len(items)-1].CreatedAt, items[len(items)-1].ID)
		nextCursor = &cursorValue
	}

	return items, nextCursor, nil
}

func (s *BookshelfService) resolveBookshelfCategories(
	ctx context.Context,
	tx *sql.Tx,
	userID uuid.UUID,
	categories []string,
) (*uuid.UUID, string, []string, error) {
	if len(categories) == 0 {
		return nil, "", nil, nil
	}

	createdNames := make([]string, 0)
	resolvedIDs := make(map[string]uuid.UUID, len(categories))
	for _, categoryName := range categories {
		var categoryID uuid.UUID
		err := tx.QueryRowContext(
			ctx,
			"SELECT id FROM bookshelf_categories WHERE user_id = $1 AND name = $2",
			userID,
			categoryName,
		).Scan(&categoryID)
		if err == nil {
			resolvedIDs[categoryName] = categoryID
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, "", nil, fmt.Errorf("failed to query bookshelf category: %w", err)
		}

		position, err := s.nextBookshelfCategoryPosition(ctx, tx, userID)
		if err != nil {
			return nil, "", nil, err
		}

		categoryID = uuid.New()
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO bookshelf_categories (id, user_id, name, position, created_at)
			VALUES ($1, $2, $3, $4, now())`,
			categoryID,
			userID,
			categoryName,
			position,
		)
		if err != nil {
			var pqErr *pq.Error
			if errors.As(err, &pqErr) && pqErr.Code == "23505" {
				if selectErr := tx.QueryRowContext(
					ctx,
					"SELECT id FROM bookshelf_categories WHERE user_id = $1 AND name = $2",
					userID,
					categoryName,
				).Scan(&categoryID); selectErr != nil {
					return nil, "", nil, fmt.Errorf("failed to resolve concurrent bookshelf category create: %w", selectErr)
				}
			} else {
				return nil, "", nil, fmt.Errorf("failed to auto-create bookshelf category: %w", err)
			}
		} else {
			createdNames = append(createdNames, categoryName)
		}

		resolvedIDs[categoryName] = categoryID
	}

	selected := categories[0]
	selectedID := resolvedIDs[selected]
	return &selectedID, selected, createdNames, nil
}

func (s *BookshelfService) nextBookshelfCategoryPosition(ctx context.Context, queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}, userID uuid.UUID) (int, error) {
	var next int
	if err := queryer.QueryRowContext(
		ctx,
		"SELECT COALESCE(MAX(position), -1) + 1 FROM bookshelf_categories WHERE user_id = $1",
		userID,
	).Scan(&next); err != nil {
		return 0, fmt.Errorf("failed to fetch next bookshelf category position: %w", err)
	}
	return next, nil
}

func (s *BookshelfService) verifyBookPost(ctx context.Context, postID uuid.UUID) error {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM posts p
			JOIN sections s ON p.section_id = s.id
			WHERE p.id = $1 AND p.deleted_at IS NULL AND s.type = 'book'
		)
	`, postID).Scan(&exists); err != nil {
		return fmt.Errorf("failed to verify book post: %w", err)
	}
	if !exists {
		return errors.New("book post not found")
	}
	return nil
}

func (s *BookshelfService) logBookshelfAudit(ctx context.Context, action string, userID uuid.UUID, metadata map[string]interface{}) error {
	if err := s.audit.LogAuditWithMetadata(ctx, action, uuid.Nil, userID, metadata); err != nil {
		return fmt.Errorf("failed to create bookshelf audit log: %w", err)
	}
	return nil
}

func scanBookshelfItem(scanner interface {
	Scan(dest ...interface{}) error
}) (*models.BookshelfItem, error) {
	var (
		item           models.BookshelfItem
		categoryID     uuid.NullUUID
		deletedAtValue sql.NullTime
	)
	if err := scanner.Scan(
		&item.ID,
		&item.UserID,
		&item.PostID,
		&categoryID,
		&item.CreatedAt,
		&deletedAtValue,
	); err != nil {
		return nil, err
	}

	if categoryID.Valid {
		item.CategoryID = &categoryID.UUID
	}
	if deletedAtValue.Valid {
		deletedAt := deletedAtValue.Time
		item.DeletedAt = &deletedAt
	}

	return &item, nil
}

func normalizeBookshelfCategoryName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("category name is required")
	}
	if strings.EqualFold(trimmed, defaultBookshelfCategoryName) {
		return "", errors.New("category name is reserved")
	}
	if len(trimmed) > maxBookshelfCategoryNameLimit {
		return "", fmt.Errorf("category name must be %d characters or less", maxBookshelfCategoryNameLimit)
	}
	return trimmed, nil
}

func normalizeBookshelfCategoriesForAdd(categories []string) ([]string, error) {
	if len(categories) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(categories))
	normalized := make([]string, 0, len(categories))
	for _, category := range categories {
		trimmed := strings.TrimSpace(category)
		if trimmed == "" || strings.EqualFold(trimmed, defaultBookshelfCategoryName) {
			continue
		}
		if len(trimmed) > maxBookshelfCategoryNameLimit {
			return nil, fmt.Errorf("category name must be %d characters or less", maxBookshelfCategoryNameLimit)
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}

	return normalized, nil
}

func uuidPointerValue(value *uuid.UUID) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func bookshelfDisplayCategoryName(name sql.NullString) string {
	if name.Valid && strings.TrimSpace(name.String) != "" {
		return name.String
	}
	return defaultBookshelfCategoryName
}

func parseBookshelfCursor(cursor string) (time.Time, uuid.UUID, bool, error) {
	parts := strings.Split(cursor, bookshelfCursorSeparator)
	switch len(parts) {
	case 1:
		createdAt, err := parseBookshelfCursorTime(parts[0])
		if err != nil {
			return time.Time{}, uuid.Nil, false, errors.New("invalid cursor")
		}
		return createdAt, uuid.Nil, false, nil
	case 2:
		createdAt, err := parseBookshelfCursorTime(parts[0])
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

func parseBookshelfCursorTime(raw string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err == nil {
		return parsed, nil
	}
	return time.Parse("2006-01-02T15:04:05.000Z07:00", raw)
}

func buildBookshelfCursor(createdAt time.Time, itemID uuid.UUID) string {
	return fmt.Sprintf("%s%s%s", createdAt.UTC().Format(time.RFC3339Nano), bookshelfCursorSeparator, itemID.String())
}
