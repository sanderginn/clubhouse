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

const (
	defaultWatchlistCategory       = "Uncategorized"
	maxWatchlistCategoryNameLength = 100
)

type watchlistPostService interface {
	GetPostByID(ctx context.Context, postID uuid.UUID, userID uuid.UUID) (*models.Post, error)
}

// WatchlistService handles watchlist operations for movie and series posts.
type WatchlistService struct {
	db          *sql.DB
	postService watchlistPostService
	audit       *AuditService
}

// NewWatchlistService creates a watchlist service with default dependencies.
func NewWatchlistService(db *sql.DB) *WatchlistService {
	return NewWatchlistServiceWithDependencies(db, NewPostService(db), NewAuditService(db))
}

// NewWatchlistServiceWithDependencies creates a watchlist service with explicit dependencies.
func NewWatchlistServiceWithDependencies(db *sql.DB, postService watchlistPostService, auditService *AuditService) *WatchlistService {
	if postService == nil {
		postService = NewPostService(db)
	}
	if auditService == nil {
		auditService = NewAuditService(db)
	}

	return &WatchlistService{
		db:          db,
		postService: postService,
		audit:       auditService,
	}
}

// AddToWatchlist saves a movie/series post in one or more categories.
func (s *WatchlistService) AddToWatchlist(ctx context.Context, userID, postID uuid.UUID, categories []string) ([]models.WatchlistItem, error) {
	ctx, span := otel.Tracer("clubhouse.watchlist").Start(ctx, "WatchlistService.AddToWatchlist")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
		attribute.Int("category_count", len(categories)),
	)
	defer span.End()

	if err := s.verifyWatchlistPost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	normalized, err := normalizeWatchlistCategories(categories)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	span.SetAttributes(attribute.StringSlice("categories", normalized))

	for _, category := range normalized {
		if category == defaultWatchlistCategory {
			continue
		}

		exists, err := s.categoryExists(ctx, userID, category)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		if !exists {
			missingErr := errors.New("category not found")
			recordSpanError(span, missingErr)
			return nil, missingErr
		}
	}

	items := make([]models.WatchlistItem, 0, len(normalized))
	changedCategories := make([]string, 0, len(normalized))

	for _, category := range normalized {
		existing, err := s.getExistingWatchlistItem(ctx, userID, postID, category)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}

		if existing != nil {
			if existing.DeletedAt != nil {
				restored, err := s.restoreWatchlistItem(ctx, existing.ID)
				if err != nil {
					recordSpanError(span, err)
					return nil, err
				}
				changedCategories = append(changedCategories, category)
				items = append(items, *restored)
				continue
			}

			items = append(items, *existing)
			continue
		}

		created, err := s.createWatchlistItem(ctx, userID, postID, category)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		changedCategories = append(changedCategories, category)
		items = append(items, *created)
	}

	if len(changedCategories) > 0 {
		if err := s.logWatchlistAudit(ctx, "add_to_watchlist", userID, map[string]interface{}{
			"post_id":    postID.String(),
			"categories": changedCategories,
		}); err != nil {
			recordSpanError(span, err)
			return nil, err
		}
	}

	return items, nil
}

// RemoveFromWatchlist removes a post from one category or all categories when category is nil.
func (s *WatchlistService) RemoveFromWatchlist(ctx context.Context, userID, postID uuid.UUID, category *string) error {
	ctx, span := otel.Tracer("clubhouse.watchlist").Start(ctx, "WatchlistService.RemoveFromWatchlist")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
		attribute.Bool("has_category", category != nil),
	)
	defer span.End()

	if err := s.verifyWatchlistPost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return err
	}

	var (
		query         string
		args          []interface{}
		auditCategory interface{}
	)

	if category == nil {
		query = `
			UPDATE watchlist_items
			SET deleted_at = now()
			WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL
		`
		args = []interface{}{userID, postID}
	} else {
		normalized, err := normalizeWatchlistCategory(*category)
		if err != nil {
			recordSpanError(span, err)
			return err
		}

		auditCategory = normalized
		span.SetAttributes(attribute.String("category", normalized))

		query = `
			UPDATE watchlist_items
			SET deleted_at = now()
			WHERE user_id = $1 AND post_id = $2 AND category = $3 AND deleted_at IS NULL
		`
		args = []interface{}{userID, postID, normalized}
	}

	result, err := s.db.ExecContext(ctx, query, args...)
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
		notFoundErr := errors.New("watchlist item not found")
		recordSpanError(span, notFoundErr)
		return notFoundErr
	}

	if err := s.logWatchlistAudit(ctx, "remove_from_watchlist", userID, map[string]interface{}{
		"post_id":  postID.String(),
		"category": auditCategory,
	}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

// GetUserWatchlist returns watchlist items grouped by category.
func (s *WatchlistService) GetUserWatchlist(ctx context.Context, userID uuid.UUID) (map[string][]models.WatchlistItemWithPost, error) {
	ctx, span := otel.Tracer("clubhouse.watchlist").Start(ctx, "WatchlistService.GetUserWatchlist")
	span.SetAttributes(attribute.String("user_id", userID.String()))
	defer span.End()

	query := `
		SELECT
			wi.id, wi.user_id, wi.post_id, wi.category, wi.created_at, wi.deleted_at
		FROM watchlist_items wi
		JOIN posts p ON wi.post_id = p.id
		WHERE wi.user_id = $1 AND wi.deleted_at IS NULL AND p.deleted_at IS NULL
		ORDER BY wi.category ASC, wi.created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	defer rows.Close()

	grouped := map[string][]models.WatchlistItemWithPost{}
	postIDs := make(map[uuid.UUID]struct{})

	for rows.Next() {
		var item models.WatchlistItem
		if err := rows.Scan(&item.ID, &item.UserID, &item.PostID, &item.Category, &item.CreatedAt, &item.DeletedAt); err != nil {
			recordSpanError(span, err)
			return nil, err
		}

		postIDs[item.PostID] = struct{}{}
		grouped[item.Category] = append(grouped[item.Category], models.WatchlistItemWithPost{
			WatchlistItem: item,
			Post:          nil,
		})
	}
	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if len(postIDs) == 0 {
		return grouped, nil
	}

	postsByID := make(map[uuid.UUID]*models.Post, len(postIDs))
	for postID := range postIDs {
		post, err := s.postService.GetPostByID(ctx, postID, userID)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		postsByID[postID] = post
	}

	for category, items := range grouped {
		for index := range items {
			if post, ok := postsByID[items[index].PostID]; ok {
				items[index].Post = post
			}
		}
		grouped[category] = items
	}

	return grouped, nil
}

// GetPostWatchlistInfo retrieves watchlist tooltip data for a post.
func (s *WatchlistService) GetPostWatchlistInfo(ctx context.Context, postID uuid.UUID, viewerID *uuid.UUID) (*models.PostWatchlistInfo, error) {
	ctx, span := otel.Tracer("clubhouse.watchlist").Start(ctx, "WatchlistService.GetPostWatchlistInfo")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.Bool("has_viewer_id", viewerID != nil),
	)
	if viewerID != nil {
		span.SetAttributes(attribute.String("viewer_id", viewerID.String()))
	}
	defer span.End()

	if err := s.verifyWatchlistPost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	var saveCount int
	countQuery := `
		SELECT COUNT(DISTINCT user_id)
		FROM watchlist_items
		WHERE post_id = $1 AND deleted_at IS NULL
	`
	if err := s.db.QueryRowContext(ctx, countQuery, postID).Scan(&saveCount); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	usersQuery := `
		SELECT u.id, u.username, u.profile_picture_url, MIN(wi.created_at) AS first_saved
		FROM watchlist_items wi
		JOIN users u ON wi.user_id = u.id
		WHERE wi.post_id = $1 AND wi.deleted_at IS NULL
		GROUP BY u.id, u.username, u.profile_picture_url
		ORDER BY first_saved ASC
	`
	rows, err := s.db.QueryContext(ctx, usersQuery, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	defer rows.Close()

	users := []models.ReactionUser{}
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
		return nil, err
	}

	info := models.PostWatchlistInfo{
		SaveCount: saveCount,
		Users:     users,
	}

	if viewerID != nil {
		viewerQuery := `
			SELECT category
			FROM watchlist_items
			WHERE post_id = $1 AND user_id = $2 AND deleted_at IS NULL
			ORDER BY created_at ASC
		`
		rows, err := s.db.QueryContext(ctx, viewerQuery, postID, *viewerID)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var viewerCategory string
			if err := rows.Scan(&viewerCategory); err != nil {
				recordSpanError(span, err)
				return nil, err
			}
			info.ViewerCategories = append(info.ViewerCategories, viewerCategory)
		}
		if err := rows.Err(); err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		if len(info.ViewerCategories) > 0 {
			info.ViewerSaved = true
		}
	}

	return &info, nil
}

// GetUserWatchlistCategories lists user watchlist categories by position.
func (s *WatchlistService) GetUserWatchlistCategories(ctx context.Context, userID uuid.UUID) ([]models.WatchlistCategory, error) {
	ctx, span := otel.Tracer("clubhouse.watchlist").Start(ctx, "WatchlistService.GetUserWatchlistCategories")
	span.SetAttributes(attribute.String("user_id", userID.String()))
	defer span.End()

	query := `
		SELECT id, user_id, name, position, created_at
		FROM watchlist_categories
		WHERE user_id = $1
		ORDER BY position ASC, created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	defer rows.Close()

	categories := []models.WatchlistCategory{}
	for rows.Next() {
		var category models.WatchlistCategory
		if err := rows.Scan(&category.ID, &category.UserID, &category.Name, &category.Position, &category.CreatedAt); err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return categories, nil
}

// CreateCategory creates a new watchlist category.
func (s *WatchlistService) CreateCategory(ctx context.Context, userID uuid.UUID, name string) (*models.WatchlistCategory, error) {
	ctx, span := otel.Tracer("clubhouse.watchlist").Start(ctx, "WatchlistService.CreateCategory")
	span.SetAttributes(attribute.String("user_id", userID.String()))
	defer span.End()

	normalized, err := normalizeWatchlistCategory(name)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	span.SetAttributes(attribute.String("category", normalized))

	position, err := s.nextCategoryPosition(ctx, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	query := `
		INSERT INTO watchlist_categories (id, user_id, name, position, created_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING id, user_id, name, position, created_at
	`

	categoryID := uuid.New()
	var category models.WatchlistCategory
	if err := s.db.QueryRowContext(ctx, query, categoryID, userID, normalized, position).Scan(
		&category.ID,
		&category.UserID,
		&category.Name,
		&category.Position,
		&category.CreatedAt,
	); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if err := s.logWatchlistAudit(ctx, "create_watchlist_category", userID, map[string]interface{}{
		"category_name": normalized,
	}); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return &category, nil
}

// UpdateCategory updates a watchlist category's name or position.
func (s *WatchlistService) UpdateCategory(ctx context.Context, userID, categoryID uuid.UUID, name *string, position *int) error {
	ctx, span := otel.Tracer("clubhouse.watchlist").Start(ctx, "WatchlistService.UpdateCategory")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("category_id", categoryID.String()),
	)
	defer span.End()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var currentName string
	if err := tx.QueryRowContext(
		ctx,
		"SELECT name FROM watchlist_categories WHERE id = $1 AND user_id = $2",
		categoryID,
		userID,
	).Scan(&currentName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := errors.New("category not found")
			recordSpanError(span, notFoundErr)
			return notFoundErr
		}
		recordSpanError(span, err)
		return err
	}

	updates := []string{}
	args := []interface{}{}
	changes := map[string]interface{}{}

	var normalizedName string
	renameCategory := false

	if name != nil {
		normalized, err := normalizeWatchlistCategory(*name)
		if err != nil {
			recordSpanError(span, err)
			return err
		}
		if normalized != currentName {
			normalizedName = normalized
			renameCategory = true
			updates = append(updates, fmt.Sprintf("name = $%d", len(args)+1))
			args = append(args, normalized)
			changes["name"] = normalized
		}
	}

	if position != nil {
		updates = append(updates, fmt.Sprintf("position = $%d", len(args)+1))
		args = append(args, *position)
		changes["position"] = *position
	}

	if len(updates) == 0 {
		invalidErr := errors.New("no updates provided")
		recordSpanError(span, invalidErr)
		return invalidErr
	}

	args = append(args, categoryID, userID)
	query := fmt.Sprintf(
		"UPDATE watchlist_categories SET %s WHERE id = $%d AND user_id = $%d",
		strings.Join(updates, ", "),
		len(args)-1,
		len(args),
	)

	result, err := tx.ExecContext(ctx, query, args...)
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
		notFoundErr := errors.New("category not found")
		recordSpanError(span, notFoundErr)
		return notFoundErr
	}

	if renameCategory {
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE watchlist_items source
			SET deleted_at = now()
			WHERE source.user_id = $1
				AND source.category = $2
				AND source.deleted_at IS NULL
				AND EXISTS (
					SELECT 1
					FROM watchlist_items target
					WHERE target.user_id = source.user_id
						AND target.post_id = source.post_id
						AND target.category = $3
						AND target.deleted_at IS NULL
				)`,
			userID,
			currentName,
			normalizedName,
		); err != nil {
			recordSpanError(span, err)
			return err
		}

		updateItems := `
			UPDATE watchlist_items
			SET category = $1
			WHERE user_id = $2 AND category = $3 AND deleted_at IS NULL
		`
		if _, err := tx.ExecContext(ctx, updateItems, normalizedName, userID, currentName); err != nil {
			recordSpanError(span, err)
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return err
	}

	if err := s.logWatchlistAudit(ctx, "update_watchlist_category", userID, map[string]interface{}{
		"category_id": categoryID.String(),
		"changes":     changes,
	}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

// DeleteCategory removes a watchlist category and moves active items to Uncategorized.
func (s *WatchlistService) DeleteCategory(ctx context.Context, userID, categoryID uuid.UUID) error {
	ctx, span := otel.Tracer("clubhouse.watchlist").Start(ctx, "WatchlistService.DeleteCategory")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("category_id", categoryID.String()),
	)
	defer span.End()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var name string
	if err := tx.QueryRowContext(
		ctx,
		"SELECT name FROM watchlist_categories WHERE id = $1 AND user_id = $2",
		categoryID,
		userID,
	).Scan(&name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := errors.New("category not found")
			recordSpanError(span, notFoundErr)
			return notFoundErr
		}
		recordSpanError(span, err)
		return err
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM watchlist_categories WHERE id = $1 AND user_id = $2", categoryID, userID); err != nil {
		recordSpanError(span, err)
		return err
	}

	if name != defaultWatchlistCategory {
		// Remove overlaps first to satisfy unique active rows on (user_id, post_id, category).
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE watchlist_items source
			SET deleted_at = now()
			WHERE source.user_id = $1
				AND source.category = $2
				AND source.deleted_at IS NULL
				AND EXISTS (
					SELECT 1
					FROM watchlist_items target
					WHERE target.user_id = source.user_id
						AND target.post_id = source.post_id
						AND target.category = $3
						AND target.deleted_at IS NULL
				)`,
			userID,
			name,
			defaultWatchlistCategory,
		); err != nil {
			recordSpanError(span, err)
			return err
		}

		if _, err := tx.ExecContext(
			ctx,
			`UPDATE watchlist_items
			SET category = $1
			WHERE user_id = $2 AND category = $3 AND deleted_at IS NULL`,
			defaultWatchlistCategory,
			userID,
			name,
		); err != nil {
			recordSpanError(span, err)
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return err
	}

	if err := s.logWatchlistAudit(ctx, "delete_watchlist_category", userID, map[string]interface{}{
		"category_id":   categoryID.String(),
		"category_name": name,
	}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (s *WatchlistService) verifyWatchlistPost(ctx context.Context, postID uuid.UUID) error {
	var exists bool
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM posts p
			JOIN sections s ON p.section_id = s.id
			WHERE p.id = $1 AND p.deleted_at IS NULL AND s.type IN ('movie', 'series')
		)
	`
	if err := s.db.QueryRowContext(ctx, query, postID).Scan(&exists); err != nil {
		return fmt.Errorf("failed to verify watchlist post: %w", err)
	}
	if !exists {
		return errors.New("movie or series post not found")
	}
	return nil
}

func (s *WatchlistService) categoryExists(ctx context.Context, userID uuid.UUID, name string) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(
		ctx,
		"SELECT EXISTS(SELECT 1 FROM watchlist_categories WHERE user_id = $1 AND name = $2)",
		userID,
		name,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check category existence: %w", err)
	}
	return exists, nil
}

func (s *WatchlistService) nextCategoryPosition(ctx context.Context, userID uuid.UUID) (int, error) {
	var next int
	if err := s.db.QueryRowContext(
		ctx,
		"SELECT COALESCE(MAX(position), -1) + 1 FROM watchlist_categories WHERE user_id = $1",
		userID,
	).Scan(&next); err != nil {
		return 0, fmt.Errorf("failed to fetch next category position: %w", err)
	}
	return next, nil
}

func (s *WatchlistService) getExistingWatchlistItem(ctx context.Context, userID, postID uuid.UUID, category string) (*models.WatchlistItem, error) {
	query := `
		SELECT id, user_id, post_id, category, created_at, deleted_at
		FROM watchlist_items
		WHERE user_id = $1 AND post_id = $2 AND category = $3
	`

	var item models.WatchlistItem
	err := s.db.QueryRowContext(ctx, query, userID, postID, category).Scan(
		&item.ID,
		&item.UserID,
		&item.PostID,
		&item.Category,
		&item.CreatedAt,
		&item.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch watchlist item: %w", err)
	}

	return &item, nil
}

func (s *WatchlistService) restoreWatchlistItem(ctx context.Context, watchlistItemID uuid.UUID) (*models.WatchlistItem, error) {
	query := `
		UPDATE watchlist_items
		SET deleted_at = NULL
		WHERE id = $1
		RETURNING id, user_id, post_id, category, created_at, deleted_at
	`

	var item models.WatchlistItem
	if err := s.db.QueryRowContext(ctx, query, watchlistItemID).Scan(
		&item.ID,
		&item.UserID,
		&item.PostID,
		&item.Category,
		&item.CreatedAt,
		&item.DeletedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to restore watchlist item: %w", err)
	}

	return &item, nil
}

func (s *WatchlistService) createWatchlistItem(ctx context.Context, userID, postID uuid.UUID, category string) (*models.WatchlistItem, error) {
	query := `
		INSERT INTO watchlist_items (id, user_id, post_id, category, created_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING id, user_id, post_id, category, created_at, deleted_at
	`

	itemID := uuid.New()
	var item models.WatchlistItem
	if err := s.db.QueryRowContext(ctx, query, itemID, userID, postID, category).Scan(
		&item.ID,
		&item.UserID,
		&item.PostID,
		&item.Category,
		&item.CreatedAt,
		&item.DeletedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to create watchlist item: %w", err)
	}

	return &item, nil
}

func (s *WatchlistService) logWatchlistAudit(ctx context.Context, action string, userID uuid.UUID, metadata map[string]interface{}) error {
	if err := s.audit.LogAuditWithMetadata(ctx, action, uuid.Nil, userID, metadata); err != nil {
		return fmt.Errorf("failed to create watchlist audit log: %w", err)
	}
	return nil
}

func normalizeWatchlistCategories(categories []string) ([]string, error) {
	if len(categories) == 0 {
		return []string{defaultWatchlistCategory}, nil
	}

	seen := map[string]struct{}{}
	normalized := []string{}
	for _, category := range categories {
		name, err := normalizeWatchlistCategory(category)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}

	if len(normalized) == 0 {
		return []string{defaultWatchlistCategory}, nil
	}

	return normalized, nil
}

func normalizeWatchlistCategory(category string) (string, error) {
	name := strings.TrimSpace(category)
	if name == "" {
		name = defaultWatchlistCategory
	}
	if len(name) > maxWatchlistCategoryNameLength {
		return "", fmt.Errorf("category name must be %d characters or less", maxWatchlistCategoryNameLength)
	}
	return name, nil
}
