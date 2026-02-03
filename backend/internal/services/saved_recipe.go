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
	defaultRecipeCategory       = "Uncategorized"
	maxRecipeCategoryNameLength = 100
)

// SavedRecipeService handles saved recipe operations.
type SavedRecipeService struct {
	db *sql.DB
}

// NewSavedRecipeService creates a new saved recipe service.
func NewSavedRecipeService(db *sql.DB) *SavedRecipeService {
	return &SavedRecipeService{db: db}
}

// SaveRecipe saves a recipe in one or more categories.
func (s *SavedRecipeService) SaveRecipe(ctx context.Context, userID, postID uuid.UUID, categories []string) ([]models.SavedRecipe, error) {
	ctx, span := otel.Tracer("clubhouse.saved_recipes").Start(ctx, "SavedRecipeService.SaveRecipe")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
		attribute.Int("category_count", len(categories)),
	)
	defer span.End()

	if err := s.verifyRecipePost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	normalized, err := normalizeRecipeCategories(categories)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	span.SetAttributes(attribute.StringSlice("categories", normalized))

	for _, category := range normalized {
		if category == defaultRecipeCategory {
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

	savedRecipes := make([]models.SavedRecipe, 0, len(normalized))
	changedCategories := []string{}

	for _, category := range normalized {
		existing, err := s.getExistingSavedRecipe(ctx, userID, postID, category)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		if existing != nil {
			if existing.DeletedAt != nil {
				restored, err := s.restoreSavedRecipe(ctx, existing.ID)
				if err != nil {
					recordSpanError(span, err)
					return nil, err
				}
				changedCategories = append(changedCategories, category)
				savedRecipes = append(savedRecipes, *restored)
				continue
			}
			savedRecipes = append(savedRecipes, *existing)
			continue
		}

		created, err := s.createSavedRecipe(ctx, userID, postID, category)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		changedCategories = append(changedCategories, category)
		savedRecipes = append(savedRecipes, *created)
	}

	if len(changedCategories) > 0 {
		if err := s.logSavedRecipeAudit(ctx, "save_recipe", userID, map[string]interface{}{
			"post_id":    postID.String(),
			"categories": changedCategories,
		}); err != nil {
			recordSpanError(span, err)
			return nil, err
		}
	}

	return savedRecipes, nil
}

// UnsaveRecipe removes saved recipes from a category or all categories when category is nil.
func (s *SavedRecipeService) UnsaveRecipe(ctx context.Context, userID, postID uuid.UUID, category *string) error {
	ctx, span := otel.Tracer("clubhouse.saved_recipes").Start(ctx, "SavedRecipeService.UnsaveRecipe")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
		attribute.Bool("has_category", category != nil),
	)
	defer span.End()

	if err := s.verifyRecipePost(ctx, postID); err != nil {
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
			UPDATE saved_recipes
			SET deleted_at = now()
			WHERE user_id = $1 AND post_id = $2 AND deleted_at IS NULL
		`
		args = []interface{}{userID, postID}
	} else {
		normalized, err := normalizeRecipeCategory(*category)
		if err != nil {
			recordSpanError(span, err)
			return err
		}
		auditCategory = normalized
		span.SetAttributes(attribute.String("category", normalized))
		query = `
			UPDATE saved_recipes
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
		notFoundErr := errors.New("saved recipe not found")
		recordSpanError(span, notFoundErr)
		return notFoundErr
	}

	if err := s.logSavedRecipeAudit(ctx, "unsave_recipe", userID, map[string]interface{}{
		"post_id":  postID.String(),
		"category": auditCategory,
	}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

// GetPostSaves retrieves save tooltip data for a post.
func (s *SavedRecipeService) GetPostSaves(ctx context.Context, postID uuid.UUID, viewerID *uuid.UUID) (*models.PostSaveInfo, error) {
	ctx, span := otel.Tracer("clubhouse.saved_recipes").Start(ctx, "SavedRecipeService.GetPostSaves")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.Bool("has_viewer_id", viewerID != nil),
	)
	if viewerID != nil {
		span.SetAttributes(attribute.String("viewer_id", viewerID.String()))
	}
	defer span.End()

	if err := s.verifyRecipePost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	var saveCount int
	countQuery := `
		SELECT COUNT(DISTINCT user_id)
		FROM saved_recipes
		WHERE post_id = $1 AND deleted_at IS NULL
	`
	if err := s.db.QueryRowContext(ctx, countQuery, postID).Scan(&saveCount); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	usersQuery := `
		SELECT u.id, u.username, u.profile_picture_url, MIN(sr.created_at) AS first_saved
		FROM saved_recipes sr
		JOIN users u ON sr.user_id = u.id
		WHERE sr.post_id = $1 AND sr.deleted_at IS NULL
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

	info := models.PostSaveInfo{
		SaveCount: saveCount,
		Users:     users,
	}

	if viewerID != nil {
		viewerQuery := `
			SELECT category
			FROM saved_recipes
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

// GetUserSavedRecipes returns saved recipes grouped by category.
func (s *SavedRecipeService) GetUserSavedRecipes(ctx context.Context, userID uuid.UUID) ([]models.SavedRecipeCategory, error) {
	ctx, span := otel.Tracer("clubhouse.saved_recipes").Start(ctx, "SavedRecipeService.GetUserSavedRecipes")
	span.SetAttributes(attribute.String("user_id", userID.String()))
	defer span.End()

	query := `
		SELECT
			sr.id, sr.user_id, sr.post_id, sr.category, sr.created_at, sr.deleted_at,
			p.id, p.user_id, p.section_id, p.content, p.created_at, p.deleted_at
		FROM saved_recipes sr
		JOIN posts p ON sr.post_id = p.id
		WHERE sr.user_id = $1 AND sr.deleted_at IS NULL AND p.deleted_at IS NULL
		ORDER BY sr.category ASC, sr.created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	defer rows.Close()

	categories := []models.SavedRecipeCategory{}
	categoryIndex := map[string]int{}

	for rows.Next() {
		var savedRecipe models.SavedRecipe
		var post models.Post
		if err := rows.Scan(
			&savedRecipe.ID, &savedRecipe.UserID, &savedRecipe.PostID, &savedRecipe.Category,
			&savedRecipe.CreatedAt, &savedRecipe.DeletedAt,
			&post.ID, &post.UserID, &post.SectionID, &post.Content, &post.CreatedAt, &post.DeletedAt,
		); err != nil {
			recordSpanError(span, err)
			return nil, err
		}

		savedWithPost := models.SavedRecipeWithPost{
			SavedRecipe: savedRecipe,
			Post:        &post,
		}

		idx, ok := categoryIndex[savedRecipe.Category]
		if !ok {
			categoryIndex[savedRecipe.Category] = len(categories)
			categories = append(categories, models.SavedRecipeCategory{
				Name:    savedRecipe.Category,
				Recipes: []models.SavedRecipeWithPost{savedWithPost},
			})
			continue
		}

		categories[idx].Recipes = append(categories[idx].Recipes, savedWithPost)
	}

	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return categories, nil
}

// GetUserCategories retrieves recipe categories for a user.
func (s *SavedRecipeService) GetUserCategories(ctx context.Context, userID uuid.UUID) ([]models.RecipeCategory, error) {
	ctx, span := otel.Tracer("clubhouse.saved_recipes").Start(ctx, "SavedRecipeService.GetUserCategories")
	span.SetAttributes(attribute.String("user_id", userID.String()))
	defer span.End()

	query := `
		SELECT id, user_id, name, position, created_at
		FROM recipe_categories
		WHERE user_id = $1
		ORDER BY position ASC, created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	defer rows.Close()

	categories := []models.RecipeCategory{}
	for rows.Next() {
		var category models.RecipeCategory
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

// CreateCategory creates a new recipe category.
func (s *SavedRecipeService) CreateCategory(ctx context.Context, userID uuid.UUID, name string) (*models.RecipeCategory, error) {
	ctx, span := otel.Tracer("clubhouse.saved_recipes").Start(ctx, "SavedRecipeService.CreateCategory")
	span.SetAttributes(attribute.String("user_id", userID.String()))
	defer span.End()

	normalized, err := normalizeRecipeCategory(name)
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
		INSERT INTO recipe_categories (id, user_id, name, position, created_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING id, user_id, name, position, created_at
	`

	categoryID := uuid.New()
	var category models.RecipeCategory
	if err := s.db.QueryRowContext(ctx, query, categoryID, userID, normalized, position).Scan(
		&category.ID, &category.UserID, &category.Name, &category.Position, &category.CreatedAt,
	); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if err := s.logSavedRecipeAudit(ctx, "create_recipe_category", userID, map[string]interface{}{
		"category_name": normalized,
	}); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return &category, nil
}

// UpdateCategory updates a recipe category's name or position.
func (s *SavedRecipeService) UpdateCategory(ctx context.Context, userID, categoryID uuid.UUID, name *string, position *int) error {
	ctx, span := otel.Tracer("clubhouse.saved_recipes").Start(ctx, "SavedRecipeService.UpdateCategory")
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
		"SELECT name FROM recipe_categories WHERE id = $1 AND user_id = $2",
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
		normalized, err := normalizeRecipeCategory(*name)
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
		"UPDATE recipe_categories SET %s WHERE id = $%d AND user_id = $%d",
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
		updateSaved := `
			UPDATE saved_recipes
			SET category = $1
			WHERE user_id = $2 AND category = $3 AND deleted_at IS NULL
		`
		if _, err := tx.ExecContext(ctx, updateSaved, normalizedName, userID, currentName); err != nil {
			recordSpanError(span, err)
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return err
	}

	if err := s.logSavedRecipeAudit(ctx, "update_recipe_category", userID, map[string]interface{}{
		"category_id": categoryID.String(),
		"changes":     changes,
	}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

// DeleteCategory removes a recipe category and moves recipes to Uncategorized.
func (s *SavedRecipeService) DeleteCategory(ctx context.Context, userID, categoryID uuid.UUID) error {
	ctx, span := otel.Tracer("clubhouse.saved_recipes").Start(ctx, "SavedRecipeService.DeleteCategory")
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
		"SELECT name FROM recipe_categories WHERE id = $1 AND user_id = $2",
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

	if _, err := tx.ExecContext(ctx, "DELETE FROM recipe_categories WHERE id = $1 AND user_id = $2", categoryID, userID); err != nil {
		recordSpanError(span, err)
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`UPDATE saved_recipes
		SET category = $1
		WHERE user_id = $2 AND category = $3 AND deleted_at IS NULL`,
		defaultRecipeCategory,
		userID,
		name,
	)
	if err != nil {
		recordSpanError(span, err)
		return err
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return err
	}

	if err := s.logSavedRecipeAudit(ctx, "delete_recipe_category", userID, map[string]interface{}{
		"category_id":   categoryID.String(),
		"category_name": name,
	}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

// verifyRecipePost ensures the post exists and belongs to the recipe section.
func (s *SavedRecipeService) verifyRecipePost(ctx context.Context, postID uuid.UUID) error {
	var exists bool
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM posts p
			JOIN sections s ON p.section_id = s.id
			WHERE p.id = $1 AND p.deleted_at IS NULL AND s.type = 'recipe'
		)
	`
	if err := s.db.QueryRowContext(ctx, query, postID).Scan(&exists); err != nil {
		return fmt.Errorf("failed to verify recipe post: %w", err)
	}
	if !exists {
		return errors.New("recipe post not found")
	}
	return nil
}

func (s *SavedRecipeService) categoryExists(ctx context.Context, userID uuid.UUID, name string) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(
		ctx,
		"SELECT EXISTS(SELECT 1 FROM recipe_categories WHERE user_id = $1 AND name = $2)",
		userID,
		name,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check category existence: %w", err)
	}
	return exists, nil
}

func (s *SavedRecipeService) nextCategoryPosition(ctx context.Context, userID uuid.UUID) (int, error) {
	var next int
	if err := s.db.QueryRowContext(
		ctx,
		"SELECT COALESCE(MAX(position), -1) + 1 FROM recipe_categories WHERE user_id = $1",
		userID,
	).Scan(&next); err != nil {
		return 0, fmt.Errorf("failed to fetch next category position: %w", err)
	}
	return next, nil
}

func (s *SavedRecipeService) getExistingSavedRecipe(ctx context.Context, userID, postID uuid.UUID, category string) (*models.SavedRecipe, error) {
	query := `
		SELECT id, user_id, post_id, category, created_at, deleted_at
		FROM saved_recipes
		WHERE user_id = $1 AND post_id = $2 AND category = $3
	`

	var savedRecipe models.SavedRecipe
	err := s.db.QueryRowContext(ctx, query, userID, postID, category).Scan(
		&savedRecipe.ID, &savedRecipe.UserID, &savedRecipe.PostID, &savedRecipe.Category,
		&savedRecipe.CreatedAt, &savedRecipe.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch saved recipe: %w", err)
	}

	return &savedRecipe, nil
}

func (s *SavedRecipeService) restoreSavedRecipe(ctx context.Context, savedRecipeID uuid.UUID) (*models.SavedRecipe, error) {
	query := `
		UPDATE saved_recipes
		SET deleted_at = NULL
		WHERE id = $1
		RETURNING id, user_id, post_id, category, created_at, deleted_at
	`

	var savedRecipe models.SavedRecipe
	if err := s.db.QueryRowContext(ctx, query, savedRecipeID).Scan(
		&savedRecipe.ID, &savedRecipe.UserID, &savedRecipe.PostID, &savedRecipe.Category,
		&savedRecipe.CreatedAt, &savedRecipe.DeletedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to restore saved recipe: %w", err)
	}

	return &savedRecipe, nil
}

func (s *SavedRecipeService) createSavedRecipe(ctx context.Context, userID, postID uuid.UUID, category string) (*models.SavedRecipe, error) {
	query := `
		INSERT INTO saved_recipes (id, user_id, post_id, category, created_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING id, user_id, post_id, category, created_at, deleted_at
	`

	savedRecipeID := uuid.New()
	var savedRecipe models.SavedRecipe
	if err := s.db.QueryRowContext(ctx, query, savedRecipeID, userID, postID, category).Scan(
		&savedRecipe.ID, &savedRecipe.UserID, &savedRecipe.PostID, &savedRecipe.Category,
		&savedRecipe.CreatedAt, &savedRecipe.DeletedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to create saved recipe: %w", err)
	}

	return &savedRecipe, nil
}

func (s *SavedRecipeService) logSavedRecipeAudit(ctx context.Context, action string, userID uuid.UUID, metadata map[string]interface{}) error {
	auditService := NewAuditService(s.db)
	if err := auditService.LogAuditWithMetadata(ctx, action, uuid.Nil, userID, metadata); err != nil {
		return fmt.Errorf("failed to create saved recipe audit log: %w", err)
	}
	return nil
}

func normalizeRecipeCategories(categories []string) ([]string, error) {
	if len(categories) == 0 {
		return []string{defaultRecipeCategory}, nil
	}

	seen := map[string]struct{}{}
	normalized := []string{}
	for _, category := range categories {
		name, err := normalizeRecipeCategory(category)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}

	if len(normalized) == 0 {
		return []string{defaultRecipeCategory}, nil
	}

	return normalized, nil
}

func normalizeRecipeCategory(category string) (string, error) {
	name := strings.TrimSpace(category)
	if name == "" {
		name = defaultRecipeCategory
	}
	if len(name) > maxRecipeCategoryNameLength {
		return "", fmt.Errorf("category name must be %d characters or less", maxRecipeCategoryNameLength)
	}
	return name, nil
}
