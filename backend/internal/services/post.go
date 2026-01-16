package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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

// GetPostByID retrieves a post by ID
func (s *PostService) GetPostByID(ctx context.Context, postID uuid.UUID) (*models.Post, error) {
	query := `
		SELECT id, user_id, section_id, content, created_at, updated_at, deleted_at
		FROM posts
		WHERE id = $1 AND deleted_at IS NULL
	`

	var post models.Post
	err := s.db.QueryRowContext(ctx, query, postID).
		Scan(&post.ID, &post.UserID, &post.SectionID, &post.Content, &post.CreatedAt, &post.UpdatedAt, &post.DeletedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("post not found")
		}
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	// Fetch links for this post
	links, err := s.getLinksForPost(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("failed to get links: %w", err)
	}
	post.Links = links

	return &post, nil
}

// getLinksForPost retrieves all links for a post
func (s *PostService) getLinksForPost(ctx context.Context, postID uuid.UUID) ([]models.Link, error) {
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
		var metadata sql.NullString

		err := rows.Scan(&link.ID, &link.URL, &metadata, &link.CreatedAt)
		if err != nil {
			return nil, err
		}

		if metadata.Valid && metadata.String != "" {
			link.Metadata = make(map[string]interface{})
			if err := json.Unmarshal([]byte(metadata.String), &link.Metadata); err != nil {
				// If metadata is invalid JSON, just skip it
				link.Metadata = nil
			}
		}

		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return links, nil
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
