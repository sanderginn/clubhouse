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
)

// PostService handles post-related operations
type PostService struct {
	db *sql.DB
}

// NewPostService creates a new post service
func NewPostService(db *sql.DB) *PostService {
	return &PostService{db: db}
}

// CreatePost creates a new post with optional links
func (s *PostService) CreatePost(ctx context.Context, req *models.CreatePostRequest, userID uuid.UUID) (*models.Post, error) {
	// Validate input
	if err := validateCreatePostInput(req); err != nil {
		return nil, err
	}

	// Parse and validate section ID
	sectionID, err := uuid.Parse(req.SectionID)
	if err != nil {
		return nil, fmt.Errorf("invalid section id")
	}

	// Verify section exists
	var sectionExists bool
	err = s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sections WHERE id = $1)", sectionID).
		Scan(&sectionExists)
	if err != nil || !sectionExists {
		return nil, fmt.Errorf("section not found")
	}

	// Create post ID
	postID := uuid.New()

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert post
	query := `
		INSERT INTO posts (id, user_id, section_id, content, created_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING id, user_id, section_id, content, created_at
	`

	var post models.Post
	err = tx.QueryRowContext(ctx, query, postID, userID, sectionID, req.Content).
		Scan(&post.ID, &post.UserID, &post.SectionID, &post.Content, &post.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}

	// Insert links if provided
	if len(req.Links) > 0 {
		post.Links = make([]models.Link, 0, len(req.Links))

		for _, linkReq := range req.Links {
			linkID := uuid.New()

			// Insert link (metadata will be fetched later)
			linkQuery := `
				INSERT INTO links (id, post_id, url, created_at)
				VALUES ($1, $2, $3, now())
				RETURNING id, url, created_at
			`

			var link models.Link
			err := tx.QueryRowContext(ctx, linkQuery, linkID, postID, linkReq.URL).
				Scan(&link.ID, &link.URL, &link.CreatedAt)

			if err != nil {
				return nil, fmt.Errorf("failed to create link: %w", err)
			}

			post.Links = append(post.Links, link)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &post, nil
}

// GetPostByID retrieves a post by ID with all related data
func (s *PostService) GetPostByID(ctx context.Context, postID uuid.UUID) (*models.Post, error) {
	query := `
		SELECT 
			p.id, p.user_id, p.section_id, p.content, 
			p.created_at, p.updated_at, p.deleted_at, p.deleted_by_user_id,
			u.id, u.username, u.email, u.profile_picture_url, u.bio, u.is_admin, u.created_at,
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

// GetFeed retrieves a paginated feed of posts for a section using cursor-based pagination
func (s *PostService) GetFeed(ctx context.Context, sectionID uuid.UUID, cursor *string, limit int) (*models.FeedResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Build base query
	query := `
		SELECT 
			p.id, p.user_id, p.section_id, p.content, 
			p.created_at, p.updated_at, p.deleted_at, p.deleted_by_user_id,
			u.id, u.username, u.email, u.profile_picture_url, u.bio, u.is_admin, u.created_at,
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
	post, err := s.GetPostByID(ctx, postID)
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
			u.id, u.username, u.email, u.profile_picture_url, u.bio, u.is_admin, u.created_at,
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

	return &post, nil
}

// GetPostsByUserID retrieves a paginated list of posts by a specific user using cursor-based pagination
func (s *PostService) GetPostsByUserID(ctx context.Context, userID uuid.UUID, cursor *string, limit int) (*models.FeedResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Build base query
	query := `
		SELECT
			p.id, p.user_id, p.section_id, p.content,
			p.created_at, p.updated_at, p.deleted_at, p.deleted_by_user_id,
			u.id, u.username, u.email, u.profile_picture_url, u.bio, u.is_admin, u.created_at,
			COALESCE(COUNT(DISTINCT c.id), 0) as comment_count
		FROM posts p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN comments c ON p.id = c.post_id AND c.deleted_at IS NULL
		WHERE p.user_id = $1 AND p.deleted_at IS NULL
	`

	args := []interface{}{userID}
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

// validateCreatePostInput validates post creation input
func validateCreatePostInput(req *models.CreatePostRequest) error {
	if strings.TrimSpace(req.SectionID) == "" {
		return fmt.Errorf("section_id is required")
	}

	if strings.TrimSpace(req.Content) == "" {
		return fmt.Errorf("content is required")
	}

	if len(req.Content) > 5000 {
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
