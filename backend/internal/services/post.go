package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// PostService handles post-related operations
type PostService struct {
	db *sql.DB
}

// NewPostService creates a new post service
func NewPostService(db *sql.DB) *PostService {
	return &PostService{db: db}
}

// GetSectionIDByPostID fetches the section id for a post.
func (s *PostService) GetSectionIDByPostID(ctx context.Context, postID uuid.UUID) (uuid.UUID, error) {
	query := `
		SELECT section_id
		FROM posts
		WHERE id = $1 AND deleted_at IS NULL
	`

	var sectionID uuid.UUID
	if err := s.db.QueryRowContext(ctx, query, postID).Scan(&sectionID); err != nil {
		return uuid.UUID{}, err
	}

	return sectionID, nil
}

// CreatePost creates a new post with optional links
func (s *PostService) CreatePost(ctx context.Context, req *models.CreatePostRequest, userID uuid.UUID) (*models.Post, error) {
	ctx, span := otel.Tracer("clubhouse.posts").Start(ctx, "PostService.CreatePost")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.Int("content_length", len(strings.TrimSpace(req.Content))),
		attribute.Bool("has_links", len(req.Links) > 0),
	)
	defer span.End()

	// Validate input
	if err := validateCreatePostInput(req); err != nil {
		return nil, err
	}

	// Parse and validate section ID
	sectionID, err := uuid.Parse(req.SectionID)
	if err != nil {
		return nil, fmt.Errorf("invalid section id")
	}
	span.SetAttributes(attribute.String("section_id", sectionID.String()))

	// Verify section exists
	var sectionExists bool
	err = s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sections WHERE id = $1)", sectionID).
		Scan(&sectionExists)
	if err != nil || !sectionExists {
		return nil, fmt.Errorf("section not found")
	}

	// Create post ID
	postID := uuid.New()
	trimmedContent := strings.TrimSpace(req.Content)

	linkMetadata := fetchLinkMetadata(ctx, req.Links)

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Insert post
	query := `
		INSERT INTO posts (id, user_id, section_id, content, created_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING id, user_id, section_id, content, created_at
	`

	var post models.Post
	err = tx.QueryRowContext(ctx, query, postID, userID, sectionID, trimmedContent).
		Scan(&post.ID, &post.UserID, &post.SectionID, &post.Content, &post.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}

	// Insert links if provided
	if len(req.Links) > 0 {
		post.Links = make([]models.Link, 0, len(req.Links))

		for i, linkReq := range req.Links {
			linkID := uuid.New()

			metadataValue := interface{}(nil)
			if len(linkMetadata) > i && len(linkMetadata[i]) > 0 {
				metadataValue = linkMetadata[i]
			}

			// Insert link
			linkQuery := `
				INSERT INTO links (id, post_id, url, metadata, created_at)
				VALUES ($1, $2, $3, $4, now())
				RETURNING id, url, created_at
			`

			var link models.Link
			err := tx.QueryRowContext(ctx, linkQuery, linkID, postID, linkReq.URL, metadataValue).
				Scan(&link.ID, &link.URL, &link.CreatedAt)

			if err != nil {
				return nil, fmt.Errorf("failed to create link: %w", err)
			}

			if meta, ok := metadataValue.(models.JSONMap); ok && len(meta) > 0 {
				link.Metadata = map[string]interface{}(meta)
			}

			post.Links = append(post.Links, link)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	observability.RecordPostCreated(ctx)
	return &post, nil
}

// GetPostByID retrieves a post by ID with all related data
func (s *PostService) GetPostByID(ctx context.Context, postID uuid.UUID, userID uuid.UUID) (*models.Post, error) {
	query := `
		SELECT
			p.id, p.user_id, p.section_id, p.content,
			p.created_at, p.updated_at, p.deleted_at, p.deleted_by_user_id,
			u.id, u.username, COALESCE(u.email, '') as email, u.profile_picture_url, u.bio, u.is_admin, u.created_at,
			COALESCE(COUNT(DISTINCT c.id), 0) as comment_count
		FROM posts p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN comments c ON p.id = c.post_id AND c.deleted_at IS NULL
		WHERE p.id = $1 AND p.deleted_at IS NULL
		GROUP BY p.id, u.id
	`

	var post models.Post
	var user models.User

	err := s.db.QueryRowContext(ctx, query, postID).Scan(
		&post.ID, &post.UserID, &post.SectionID, &post.Content,
		&post.CreatedAt, &post.UpdatedAt, &post.DeletedAt, &post.DeletedByUserID,
		&user.ID, &user.Username, &user.Email, &user.ProfilePictureURL, &user.Bio, &user.IsAdmin, &user.CreatedAt,
		&post.CommentCount,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("post not found")
		}
		return nil, err
	}

	post.User = &user

	// Fetch links for this post
	links, err := s.getPostLinks(ctx, postID)
	if err != nil {
		return nil, err
	}
	post.Links = links

	// Fetch reactions
	counts, viewerReactions, err := s.getPostReactions(ctx, postID, userID)
	if err != nil {
		return nil, err
	}
	post.ReactionCounts = counts
	post.ViewerReactions = viewerReactions

	return &post, nil
}

// getPostLinks retrieves all links for a post
func (s *PostService) getPostLinks(ctx context.Context, postID uuid.UUID) ([]models.Link, error) {
	query := `
		SELECT id, url, metadata, created_at
		FROM links
		WHERE post_id = $1
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []models.Link
	for rows.Next() {
		var link models.Link
		var metadataJSON sql.NullString

		err := rows.Scan(&link.ID, &link.URL, &metadataJSON, &link.CreatedAt)
		if err != nil {
			return nil, err
		}

		// Parse metadata if present
		if metadataJSON.Valid {
			err := json.Unmarshal([]byte(metadataJSON.String), &link.Metadata)
			if err != nil {
				// If metadata is invalid JSON, just skip it
				link.Metadata = nil
			}
		}

		links = append(links, link)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return links, nil
}

// getPostReactions retrieves reaction counts and viewer reactions for a post
func (s *PostService) getPostReactions(ctx context.Context, postID uuid.UUID, viewerID uuid.UUID) (map[string]int, []string, error) {
	// Get counts
	rows, err := s.db.QueryContext(ctx, `
		SELECT emoji, COUNT(*)
		FROM reactions
		WHERE post_id = $1 AND deleted_at IS NULL
		GROUP BY emoji
	`, postID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var emoji string
		var count int
		if err := rows.Scan(&emoji, &count); err != nil {
			return nil, nil, err
		}
		counts[emoji] = count
	}

	var viewerReactions []string
	if viewerID != uuid.Nil {
		rows, err := s.db.QueryContext(ctx, `
			SELECT emoji
			FROM reactions
			WHERE post_id = $1 AND user_id = $2 AND deleted_at IS NULL
		`, postID, viewerID)
		if err != nil {
			return nil, nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var emoji string
			if err := rows.Scan(&emoji); err != nil {
				return nil, nil, err
			}
			viewerReactions = append(viewerReactions, emoji)
		}
	}

	return counts, viewerReactions, nil
}

// GetFeed retrieves a paginated feed of posts for a section using cursor-based pagination
func (s *PostService) GetFeed(ctx context.Context, sectionID uuid.UUID, cursor *string, limit int, userID uuid.UUID) (*models.FeedResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Build base query
	query := `
		SELECT
			p.id, p.user_id, p.section_id, p.content,
			p.created_at, p.updated_at, p.deleted_at, p.deleted_by_user_id,
			u.id, u.username, COALESCE(u.email, '') as email, u.profile_picture_url, u.bio, u.is_admin, u.created_at,
			COALESCE(COUNT(DISTINCT c.id), 0) as comment_count
		FROM posts p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN comments c ON p.id = c.post_id AND c.deleted_at IS NULL
		WHERE p.section_id = $1 AND p.deleted_at IS NULL
	`

	args := []interface{}{sectionID}
	argIndex := 2

	// Apply cursor if provided (cursor is the created_at timestamp from the last post)
	if cursor != nil && *cursor != "" {
		query += fmt.Sprintf(" AND p.created_at < $%d", argIndex)
		args = append(args, *cursor)
		argIndex++
	}

	query += fmt.Sprintf(" GROUP BY p.id, u.id ORDER BY p.created_at DESC LIMIT $%d", argIndex)
	args = append(args, limit+1) // Fetch one extra to determine if hasMore

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*models.Post
	for rows.Next() {
		var post models.Post
		var user models.User

		err := rows.Scan(
			&post.ID, &post.UserID, &post.SectionID, &post.Content,
			&post.CreatedAt, &post.UpdatedAt, &post.DeletedAt, &post.DeletedByUserID,
			&user.ID, &user.Username, &user.Email, &user.ProfilePictureURL, &user.Bio, &user.IsAdmin, &user.CreatedAt,
			&post.CommentCount,
		)
		if err != nil {
			return nil, err
		}

		post.User = &user

		// Fetch links for this post
		links, err := s.getPostLinks(ctx, post.ID)
		if err != nil {
			return nil, err
		}
		post.Links = links

		// Fetch reactions
		counts, viewerReactions, err := s.getPostReactions(ctx, post.ID, userID)
		if err != nil {
			return nil, err
		}
		post.ReactionCounts = counts
		post.ViewerReactions = viewerReactions

		posts = append(posts, &post)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	// Determine if there are more posts
	hasMore := len(posts) > limit
	if hasMore {
		posts = posts[:limit] // Trim to the requested limit
	}

	// Determine next cursor
	var nextCursor *string
	if hasMore && len(posts) > 0 {
		// Next cursor is the created_at of the last post in the result
		lastPost := posts[len(posts)-1]
		cursorStr := lastPost.CreatedAt.Format("2006-01-02T15:04:05.000Z07:00")
		nextCursor = &cursorStr
	}

	return &models.FeedResponse{
		Posts:      posts,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

// DeletePost soft-deletes a post (only post owner or admin can delete)
func (s *PostService) DeletePost(ctx context.Context, postID uuid.UUID, userID uuid.UUID, isAdmin bool) (*models.Post, error) {
	// Fetch the post to verify ownership
	post, err := s.GetPostByID(ctx, postID, userID)
	if err != nil {
		if err.Error() == "post not found" {
			return nil, errors.New("post not found")
		}
		return nil, err
	}

	// Check authorization: owner or admin can delete
	if post.UserID != userID && !isAdmin {
		return nil, errors.New("unauthorized to delete this post")
	}

	// Soft delete the post
	query := `
		UPDATE posts
		SET deleted_at = now(), deleted_by_user_id = $1
		WHERE id = $2
		RETURNING id, user_id, section_id, content, created_at, updated_at, deleted_at, deleted_by_user_id
	`

	var updatedPost models.Post
	err = s.db.QueryRowContext(ctx, query, userID, postID).Scan(
		&updatedPost.ID, &updatedPost.UserID, &updatedPost.SectionID, &updatedPost.Content,
		&updatedPost.CreatedAt, &updatedPost.UpdatedAt, &updatedPost.DeletedAt, &updatedPost.DeletedByUserID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to delete post: %w", err)
	}

	// Copy over the user and links from the original post
	updatedPost.User = post.User
	updatedPost.Links = post.Links
	updatedPost.ReactionCounts = post.ReactionCounts
	updatedPost.ViewerReactions = post.ViewerReactions
	observability.RecordPostDeleted(ctx)

	return &updatedPost, nil
}

// RestorePost restores a soft-deleted post
// Only the post owner (within 7 days) or an admin can restore
func (s *PostService) RestorePost(ctx context.Context, postID uuid.UUID, userID uuid.UUID, isAdmin bool) (*models.Post, error) {
	// First, fetch the deleted post
	query := `
		SELECT
			p.id, p.user_id, p.section_id, p.content,
			p.created_at, p.updated_at, p.deleted_at, p.deleted_by_user_id,
			u.id, u.username, COALESCE(u.email, '') as email, u.profile_picture_url, u.bio, u.is_admin, u.created_at,
			COALESCE(COUNT(DISTINCT c.id), 0) as comment_count
		FROM posts p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN comments c ON p.id = c.post_id AND c.deleted_at IS NULL
		WHERE p.id = $1 AND p.deleted_at IS NOT NULL
		GROUP BY p.id, u.id
	`

	var post models.Post
	var user models.User

	err := s.db.QueryRowContext(ctx, query, postID).Scan(
		&post.ID, &post.UserID, &post.SectionID, &post.Content,
		&post.CreatedAt, &post.UpdatedAt, &post.DeletedAt, &post.DeletedByUserID,
		&user.ID, &user.Username, &user.Email, &user.ProfilePictureURL, &user.Bio, &user.IsAdmin, &user.CreatedAt,
		&post.CommentCount,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("post not found")
		}
		return nil, err
	}

	// Check permissions
	// Only owner (within 7 days) or admin can restore
	if !isAdmin && post.UserID != userID {
		return nil, errors.New("unauthorized")
	}

	if !isAdmin && post.DeletedAt != nil {
		// Check if within 7 days
		sevenDaysAgo := time.Now().AddDate(0, 0, -7)
		if post.DeletedAt.Before(sevenDaysAgo) {
			return nil, errors.New("post permanently deleted")
		}
	}

	// Restore the post (clear deleted_at and deleted_by_user_id)
	updateQuery := `
		UPDATE posts
		SET deleted_at = NULL, deleted_by_user_id = NULL
		WHERE id = $1
		RETURNING id, user_id, section_id, content, created_at, updated_at, deleted_at, deleted_by_user_id
	`

	err = s.db.QueryRowContext(ctx, updateQuery, postID).Scan(
		&post.ID, &post.UserID, &post.SectionID, &post.Content,
		&post.CreatedAt, &post.UpdatedAt, &post.DeletedAt, &post.DeletedByUserID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to restore post: %w", err)
	}

	post.User = &user

	// Fetch links for this post
	links, err := s.getPostLinks(ctx, postID)
	if err != nil {
		return nil, err
	}
	post.Links = links

	// Fetch reactions
	counts, viewerReactions, err := s.getPostReactions(ctx, postID, userID)
	if err != nil {
		return nil, err
	}
	post.ReactionCounts = counts
	post.ViewerReactions = viewerReactions
	observability.RecordPostRestored(ctx)

	return &post, nil
}

// GetPostsByUserID retrieves a paginated list of posts by a specific user using cursor-based pagination
func (s *PostService) GetPostsByUserID(ctx context.Context, targetUserID uuid.UUID, cursor *string, limit int, viewerID uuid.UUID) (*models.FeedResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Build base query
	query := `
		SELECT
			p.id, p.user_id, p.section_id, p.content,
			p.created_at, p.updated_at, p.deleted_at, p.deleted_by_user_id,
			u.id, u.username, COALESCE(u.email, '') as email, u.profile_picture_url, u.bio, u.is_admin, u.created_at,
			COALESCE(COUNT(DISTINCT c.id), 0) as comment_count
		FROM posts p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN comments c ON p.id = c.post_id AND c.deleted_at IS NULL
		WHERE p.user_id = $1 AND p.deleted_at IS NULL
	`

	args := []interface{}{targetUserID}
	argIndex := 2

	// Apply cursor if provided (cursor is the created_at timestamp from the last post)
	if cursor != nil && *cursor != "" {
		query += fmt.Sprintf(" AND p.created_at < $%d", argIndex)
		args = append(args, *cursor)
		argIndex++
	}

	query += fmt.Sprintf(" GROUP BY p.id, u.id ORDER BY p.created_at DESC LIMIT $%d", argIndex)
	args = append(args, limit+1) // Fetch one extra to determine if hasMore

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*models.Post
	for rows.Next() {
		var post models.Post
		var user models.User

		err := rows.Scan(
			&post.ID, &post.UserID, &post.SectionID, &post.Content,
			&post.CreatedAt, &post.UpdatedAt, &post.DeletedAt, &post.DeletedByUserID,
			&user.ID, &user.Username, &user.Email, &user.ProfilePictureURL, &user.Bio, &user.IsAdmin, &user.CreatedAt,
			&post.CommentCount,
		)
		if err != nil {
			return nil, err
		}

		post.User = &user

		// Fetch links for this post
		links, err := s.getPostLinks(ctx, post.ID)
		if err != nil {
			return nil, err
		}
		post.Links = links

		// Fetch reactions
		counts, viewerReactions, err := s.getPostReactions(ctx, post.ID, viewerID)
		if err != nil {
			return nil, err
		}
		post.ReactionCounts = counts
		post.ViewerReactions = viewerReactions

		posts = append(posts, &post)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	// Determine if there are more posts
	hasMore := len(posts) > limit
	if hasMore {
		posts = posts[:limit] // Trim to the requested limit
	}

	// Determine next cursor
	var nextCursor *string
	if hasMore && len(posts) > 0 {
		// Next cursor is the created_at of the last post in the result
		lastPost := posts[len(posts)-1]
		cursorStr := lastPost.CreatedAt.Format("2006-01-02T15:04:05.000Z07:00")
		nextCursor = &cursorStr
	}

	return &models.FeedResponse{
		Posts:      posts,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

// HardDeletePost permanently deletes a post and all related data (admin only)
func (s *PostService) HardDeletePost(ctx context.Context, postID uuid.UUID, adminUserID uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Verify post exists (include soft-deleted posts)
	var exists bool
	err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1)", postID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check post existence: %w", err)
	}
	if !exists {
		return ErrPostNotFound
	}

	// Create audit log entry BEFORE deleting the post (FK constraint)
	auditQuery := `
		INSERT INTO audit_logs (admin_user_id, action, related_post_id, created_at)
		VALUES ($1, 'hard_delete_post', $2, now())
	`
	_, err = tx.ExecContext(ctx, auditQuery, adminUserID, postID)
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	// Delete links associated with comments on this post
	_, err = tx.ExecContext(ctx, "DELETE FROM links WHERE comment_id IN (SELECT id FROM comments WHERE post_id = $1)", postID)
	if err != nil {
		return fmt.Errorf("failed to delete comment links: %w", err)
	}

	// Delete reactions on comments of this post
	_, err = tx.ExecContext(ctx, "DELETE FROM reactions WHERE comment_id IN (SELECT id FROM comments WHERE post_id = $1)", postID)
	if err != nil {
		return fmt.Errorf("failed to delete comment reactions: %w", err)
	}

	// Delete mentions from comments on this post
	_, err = tx.ExecContext(ctx, "DELETE FROM mentions WHERE comment_id IN (SELECT id FROM comments WHERE post_id = $1)", postID)
	if err != nil {
		return fmt.Errorf("failed to delete comment mentions: %w", err)
	}

	// Delete notifications related to this post or its comments
	_, err = tx.ExecContext(ctx, "DELETE FROM notifications WHERE related_post_id = $1 OR related_comment_id IN (SELECT id FROM comments WHERE post_id = $1)", postID)
	if err != nil {
		return fmt.Errorf("failed to delete notifications: %w", err)
	}

	// Delete comments on this post
	_, err = tx.ExecContext(ctx, "DELETE FROM comments WHERE post_id = $1", postID)
	if err != nil {
		return fmt.Errorf("failed to delete comments: %w", err)
	}

	// Delete reactions on this post
	_, err = tx.ExecContext(ctx, "DELETE FROM reactions WHERE post_id = $1", postID)
	if err != nil {
		return fmt.Errorf("failed to delete post reactions: %w", err)
	}

	// Delete mentions from this post
	_, err = tx.ExecContext(ctx, "DELETE FROM mentions WHERE post_id = $1", postID)
	if err != nil {
		return fmt.Errorf("failed to delete post mentions: %w", err)
	}

	// Delete links associated with this post
	_, err = tx.ExecContext(ctx, "DELETE FROM links WHERE post_id = $1", postID)
	if err != nil {
		return fmt.Errorf("failed to delete post links: %w", err)
	}

	// Delete the post
	result, err := tx.ExecContext(ctx, "DELETE FROM posts WHERE id = $1", postID)
	if err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrPostNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	observability.RecordPostDeleted(ctx)

	return nil
}

// AdminRestorePost restores a soft-deleted post (admin only) with audit logging
func (s *PostService) AdminRestorePost(ctx context.Context, postID uuid.UUID, adminUserID uuid.UUID) (*models.Post, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Check if post exists and is soft-deleted
	var exists bool
	var isDeleted bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1),
		       EXISTS(SELECT 1 FROM posts WHERE id = $1 AND deleted_at IS NOT NULL)
	`, postID).Scan(&exists, &isDeleted)
	if err != nil {
		return nil, fmt.Errorf("failed to check post: %w", err)
	}
	if !exists {
		return nil, ErrPostNotFound
	}
	if !isDeleted {
		return nil, errors.New("post is not deleted")
	}

	// Restore the post
	var post models.Post
	err = tx.QueryRowContext(ctx, `
		UPDATE posts
		SET deleted_at = NULL, deleted_by_user_id = NULL
		WHERE id = $1
		RETURNING id, user_id, section_id, content, created_at, updated_at, deleted_at, deleted_by_user_id
	`, postID).Scan(
		&post.ID, &post.UserID, &post.SectionID, &post.Content,
		&post.CreatedAt, &post.UpdatedAt, &post.DeletedAt, &post.DeletedByUserID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to restore post: %w", err)
	}

	// Create audit log entry
	_, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs (admin_user_id, action, related_post_id, created_at)
		VALUES ($1, 'restore_post', $2, now())
	`, adminUserID, postID)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Fetch the full post with user info
	fullPost, err := s.GetPostByID(ctx, postID, adminUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch restored post: %w", err)
	}
	observability.RecordPostRestored(ctx)

	return fullPost, nil
}

// validateCreatePostInput validates post creation input
func validateCreatePostInput(req *models.CreatePostRequest) error {
	if strings.TrimSpace(req.SectionID) == "" {
		return fmt.Errorf("section_id is required")
	}

	trimmedContent := strings.TrimSpace(req.Content)
	if trimmedContent == "" && len(req.Links) == 0 {
		return fmt.Errorf("content is required")
	}

	if len(trimmedContent) > 5000 {
		return fmt.Errorf("content must be less than 5000 characters")
	}

	// Validate links if provided
	for _, link := range req.Links {
		if strings.TrimSpace(link.URL) == "" {
			return fmt.Errorf("link url cannot be empty")
		}
		if len(link.URL) > 2048 {
			return fmt.Errorf("link url must be less than 2048 characters")
		}
	}

	return nil
}
