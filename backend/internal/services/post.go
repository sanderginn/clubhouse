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

const maxPostImages = 10

// NewPostService creates a new post service
func NewPostService(db *sql.DB) *PostService {
	return &PostService{db: db}
}

// GetSectionIDByPostID fetches the section id for a post.
func (s *PostService) GetSectionIDByPostID(ctx context.Context, postID uuid.UUID) (uuid.UUID, error) {
	ctx, span := otel.Tracer("clubhouse.posts").Start(ctx, "PostService.GetSectionIDByPostID")
	span.SetAttributes(attribute.String("post_id", postID.String()))
	defer span.End()

	query := `
		SELECT section_id
		FROM posts
		WHERE id = $1 AND deleted_at IS NULL
	`

	var sectionID uuid.UUID
	if err := s.db.QueryRowContext(ctx, query, postID).Scan(&sectionID); err != nil {
		recordSpanError(span, err)
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
		attribute.Bool("has_images", len(req.Images) > 0),
		attribute.Int("image_count", len(req.Images)),
	)
	defer span.End()

	// Validate input
	if err := validateCreatePostInput(req); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	// Parse and validate section ID
	sectionID, err := uuid.Parse(req.SectionID)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("invalid section id")
	}
	span.SetAttributes(attribute.String("section_id", sectionID.String()))

	// Verify section exists and load name/type for metrics and link validation
	var sectionName string
	var sectionType string
	err = s.db.QueryRowContext(ctx, "SELECT name, type FROM sections WHERE id = $1", sectionID).
		Scan(&sectionName, &sectionType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf("section not found")
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("section not found")
	}

	for _, link := range req.Links {
		if err := models.ValidateHighlights(sectionType, link.Highlights); err != nil {
			recordSpanError(span, err)
			return nil, err
		}
	}

	// Create post ID
	postID := uuid.New()
	trimmedContent := strings.TrimSpace(req.Content)

	linkMetadata := fetchLinkMetadata(ctx, req.Links)

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
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
		recordSpanError(span, err)
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
				recordSpanError(span, err)
				return nil, fmt.Errorf("failed to create link: %w", err)
			}

			if meta, ok := metadataValue.(models.JSONMap); ok && len(meta) > 0 {
				link.Metadata = map[string]interface{}(meta)
			}

			post.Links = append(post.Links, link)
		}
	}

	// Insert images if provided
	if len(req.Images) > 0 {
		post.Images = make([]models.PostImage, 0, len(req.Images))

		for i, imageReq := range req.Images {
			imageReq = normalizePostImageRequest(imageReq)
			imageID := uuid.New()
			position := i
			captionValue := interface{}(nil)
			if imageReq.Caption != nil {
				captionValue = *imageReq.Caption
			}
			altValue := interface{}(nil)
			if imageReq.AltText != nil {
				altValue = *imageReq.AltText
			}

			imageQuery := `
				INSERT INTO post_images (id, post_id, image_url, position, caption, alt_text, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, now())
				RETURNING id, image_url, position, caption, alt_text, created_at
			`

			var image models.PostImage
			var captionDB sql.NullString
			var altDB sql.NullString
			err := tx.QueryRowContext(ctx, imageQuery, imageID, postID, imageReq.URL, position, captionValue, altValue).
				Scan(&image.ID, &image.URL, &image.Position, &captionDB, &altDB, &image.CreatedAt)

			if err != nil {
				recordSpanError(span, err)
				return nil, fmt.Errorf("failed to create post image: %w", err)
			}

			if captionDB.Valid {
				image.Caption = &captionDB.String
			}
			if altDB.Valid {
				image.AltText = &altDB.String
			}

			post.Images = append(post.Images, image)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	observability.RecordPostCreated(ctx, sectionName)
	return &post, nil
}

// UpdatePost updates a post's content and links (author only).

func (s *PostService) UpdatePost(ctx context.Context, postID uuid.UUID, userID uuid.UUID, req *models.UpdatePostRequest) (*models.Post, error) {
	ctx, span := otel.Tracer("clubhouse.posts").Start(ctx, "PostService.UpdatePost")
	defer span.End()

	if err := validateUpdatePostInput(req); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
		attribute.Int("content_length", len(strings.TrimSpace(req.Content))),
		attribute.Bool("has_links", req.Links != nil && len(*req.Links) > 0),
		attribute.Bool("has_images", req.Images != nil && len(*req.Images) > 0),
		attribute.Int("image_count", imageCount(req.Images)),
	)

	trimmedContent := strings.TrimSpace(req.Content)
	var linkMetadata []models.JSONMap
	linksChanged := false
	imagesChanged := false
	var normalizedImages []models.PostImageRequest

	var ownerID uuid.UUID
	var previousContent string
	var sectionID uuid.UUID
	var sectionType string
	err := s.db.QueryRowContext(ctx, `
		SELECT p.user_id, p.content, p.section_id, s.type
		FROM posts p
		JOIN sections s ON p.section_id = s.id
		WHERE p.id = $1 AND p.deleted_at IS NULL
	`, postID).Scan(&ownerID, &previousContent, &sectionID, &sectionType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := errors.New("post not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to fetch post owner: %w", err)
	}

	if ownerID != userID {
		unauthorizedErr := errors.New("unauthorized to edit this post")
		recordSpanError(span, unauthorizedErr)
		return nil, unauthorizedErr
	}

	if req.Links != nil {
		for _, link := range *req.Links {
			if err := models.ValidateHighlights(sectionType, link.Highlights); err != nil {
				recordSpanError(span, err)
				return nil, err
			}
		}

		existingURLs, err := getPostLinkURLs(ctx, s.db, postID)
		if err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to fetch post links: %w", err)
		}

		linksChanged = !linkRequestsMatchURLs(existingURLs, *req.Links)
		if linksChanged && len(*req.Links) > 0 {
			linkMetadata = fetchLinkMetadata(ctx, *req.Links)
		}
	}

	if req.Images != nil {
		normalizedImages = normalizePostImageRequests(*req.Images)
		existingImages, err := getPostImageEntries(ctx, s.db, postID)
		if err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to fetch post images: %w", err)
		}

		imagesChanged = !postImageRequestsMatchEntries(existingImages, normalizedImages)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.ExecContext(ctx, `
		UPDATE posts
		SET content = $1, updated_at = now()
		WHERE id = $2
	`, trimmedContent, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to update post: %w", err)
	}

	if req.Links != nil && linksChanged {
		if _, err := tx.ExecContext(ctx, "DELETE FROM links WHERE post_id = $1", postID); err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to delete post links: %w", err)
		}

		if len(*req.Links) > 0 {
			for i, linkReq := range *req.Links {
				linkID := uuid.New()

				metadataValue := interface{}(nil)
				if len(linkMetadata) > i && len(linkMetadata[i]) > 0 {
					metadataValue = linkMetadata[i]
				}

				_, err := tx.ExecContext(ctx, `
					INSERT INTO links (id, post_id, url, metadata, created_at)
					VALUES ($1, $2, $3, $4, now())
				`, linkID, postID, linkReq.URL, metadataValue)
				if err != nil {
					recordSpanError(span, err)
					return nil, fmt.Errorf("failed to create link: %w", err)
				}
			}
		}
	}

	if req.Images != nil && imagesChanged {
		if _, err := tx.ExecContext(ctx, "DELETE FROM post_images WHERE post_id = $1", postID); err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to delete post images: %w", err)
		}

		if len(normalizedImages) > 0 {
			for i, imageReq := range normalizedImages {
				captionValue := interface{}(nil)
				if imageReq.Caption != nil {
					captionValue = *imageReq.Caption
				}
				altValue := interface{}(nil)
				if imageReq.AltText != nil {
					altValue = *imageReq.AltText
				}

				_, err := tx.ExecContext(ctx, `
					INSERT INTO post_images (id, post_id, image_url, position, caption, alt_text, created_at)
					VALUES ($1, $2, $3, $4, $5, $6, now())
				`, uuid.New(), postID, imageReq.URL, i, captionValue, altValue)
				if err != nil {
					recordSpanError(span, err)
					return nil, fmt.Errorf("failed to create post image: %w", err)
				}
			}
		}
	}

	metadata := map[string]interface{}{
		"post_id":          postID.String(),
		"section_id":       sectionID.String(),
		"content_excerpt":  truncateAuditExcerpt(trimmedContent),
		"previous_content": previousContent,
		"links_changed":    linksChanged,
		"links_provided":   req.Links != nil,
		"images_changed":   imagesChanged,
		"images_provided":  req.Images != nil,
	}
	if req.Links != nil {
		metadata["link_count"] = len(*req.Links)
	}
	if req.Images != nil {
		metadata["image_count"] = len(*req.Images)
	}

	auditService := NewAuditService(tx)
	if err := auditService.LogAuditWithMetadata(ctx, "update_post", userID, ownerID, metadata); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return s.GetPostByID(ctx, postID, userID)
}

// GetPostByID retrieves a post by ID with all related data
func (s *PostService) GetPostByID(ctx context.Context, postID uuid.UUID, userID uuid.UUID) (*models.Post, error) {
	ctx, span := otel.Tracer("clubhouse.posts").Start(ctx, "PostService.GetPostByID")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.String("user_id", userID.String()),
	)
	defer span.End()

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
			notFoundErr := errors.New("post not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, err
	}

	post.User = &user

	// Fetch links for this post
	links, err := s.getPostLinks(ctx, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	post.Links = links

	// Fetch images for this post
	images, err := s.getPostImages(ctx, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	post.Images = images

	// Fetch reactions
	counts, viewerReactions, err := s.getPostReactions(ctx, postID, userID)
	if err != nil {
		recordSpanError(span, err)
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

// getPostImages retrieves all images for a post in order.
func (s *PostService) getPostImages(ctx context.Context, postID uuid.UUID) ([]models.PostImage, error) {
	query := `
		SELECT id, image_url, position, caption, alt_text, created_at
		FROM post_images
		WHERE post_id = $1
		ORDER BY position ASC
	`

	rows, err := s.db.QueryContext(ctx, query, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []models.PostImage
	for rows.Next() {
		var image models.PostImage
		var caption sql.NullString
		var altText sql.NullString

		if err := rows.Scan(&image.ID, &image.URL, &image.Position, &caption, &altText, &image.CreatedAt); err != nil {
			return nil, err
		}

		if caption.Valid {
			image.Caption = &caption.String
		}
		if altText.Valid {
			image.AltText = &altText.String
		}

		images = append(images, image)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return images, nil
}

func getPostLinkURLs(ctx context.Context, queryer interface {
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
}, postID uuid.UUID) ([]string, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT url
		FROM links
		WHERE post_id = $1
		ORDER BY created_at ASC
	`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var urls []string
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, err
		}
		urls = append(urls, url)
	}

	return urls, rows.Err()
}

type postImageEntry struct {
	url     string
	caption sql.NullString
	altText sql.NullString
}

func getPostImageEntries(ctx context.Context, queryer interface {
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
}, postID uuid.UUID) ([]postImageEntry, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT image_url, caption, alt_text
		FROM post_images
		WHERE post_id = $1
		ORDER BY position ASC
	`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []postImageEntry
	for rows.Next() {
		var entry postImageEntry
		if err := rows.Scan(&entry.url, &entry.caption, &entry.altText); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

func postImageRequestsMatchEntries(existing []postImageEntry, req []models.PostImageRequest) bool {
	if len(existing) != len(req) {
		return false
	}

	for i, entry := range existing {
		if entry.url != req[i].URL {
			return false
		}
		if !optionalTextMatches(entry.caption, req[i].Caption) {
			return false
		}
		if !optionalTextMatches(entry.altText, req[i].AltText) {
			return false
		}
	}

	return true
}

func optionalTextMatches(value sql.NullString, expected *string) bool {
	if expected == nil {
		return !value.Valid
	}
	return value.Valid && value.String == *expected
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
	ctx, span := otel.Tracer("clubhouse.posts").Start(ctx, "PostService.GetFeed")
	span.SetAttributes(
		attribute.String("section_id", sectionID.String()),
		attribute.String("user_id", userID.String()),
		attribute.Int("limit", limit),
		attribute.Bool("has_cursor", cursor != nil && *cursor != ""),
	)
	defer span.End()

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
		recordSpanError(span, err)
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
			recordSpanError(span, err)
			return nil, err
		}

		post.User = &user

		// Fetch links for this post
		links, err := s.getPostLinks(ctx, post.ID)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		post.Links = links

		// Fetch images for this post
		images, err := s.getPostImages(ctx, post.ID)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		post.Images = images

		// Fetch reactions
		counts, viewerReactions, err := s.getPostReactions(ctx, post.ID, userID)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		post.ReactionCounts = counts
		post.ViewerReactions = viewerReactions

		posts = append(posts, &post)
	}

	if err = rows.Err(); err != nil {
		recordSpanError(span, err)
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
	ctx, span := otel.Tracer("clubhouse.posts").Start(ctx, "PostService.DeletePost")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.String("user_id", userID.String()),
		attribute.Bool("is_admin", isAdmin),
	)
	defer span.End()

	// Fetch the post to verify ownership
	post, err := s.GetPostByID(ctx, postID, userID)
	if err != nil {
		if err.Error() == "post not found" {
			notFoundErr := errors.New("post not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, err
	}

	// Check authorization: owner or admin can delete
	if post.UserID != userID && !isAdmin {
		unauthorizedErr := errors.New("unauthorized to delete this post")
		recordSpanError(span, unauthorizedErr)
		return nil, unauthorizedErr
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Soft delete the post
	query := `
		UPDATE posts
		SET deleted_at = now(), deleted_by_user_id = $1
		WHERE id = $2
		RETURNING id, user_id, section_id, content, created_at, updated_at, deleted_at, deleted_by_user_id
	`

	var updatedPost models.Post
	err = tx.QueryRowContext(ctx, query, userID, postID).Scan(
		&updatedPost.ID, &updatedPost.UserID, &updatedPost.SectionID, &updatedPost.Content,
		&updatedPost.CreatedAt, &updatedPost.UpdatedAt, &updatedPost.DeletedAt, &updatedPost.DeletedByUserID,
	)

	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to delete post: %w", err)
	}

	isSelfDelete := post.UserID == userID
	auditService := NewAuditService(tx)
	metadata := map[string]interface{}{
		"post_id":            post.ID.String(),
		"section_id":         post.SectionID.String(),
		"content_excerpt":    truncateAuditExcerpt(post.Content),
		"deleted_by_user_id": userID.String(),
		"is_self_delete":     isSelfDelete,
	}
	if !isSelfDelete && isAdmin {
		metadata["deleted_by_admin"] = true
	}
	if err := auditService.LogModerationAudit(
		ctx,
		"delete_post",
		userID,
		post.UserID,
		post.ID,
		uuid.Nil,
		metadata,
	); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Copy over the user and links from the original post
	updatedPost.User = post.User
	updatedPost.Links = post.Links
	updatedPost.Images = post.Images
	updatedPost.ReactionCounts = post.ReactionCounts
	updatedPost.ViewerReactions = post.ViewerReactions
	observability.RecordPostDeleted(ctx)

	return &updatedPost, nil
}

// RestorePost restores a soft-deleted post
// Only the post owner (within 7 days) or an admin can restore
func (s *PostService) RestorePost(ctx context.Context, postID uuid.UUID, userID uuid.UUID, isAdmin bool) (*models.Post, error) {
	ctx, span := otel.Tracer("clubhouse.posts").Start(ctx, "PostService.RestorePost")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.String("user_id", userID.String()),
		attribute.Bool("is_admin", isAdmin),
	)
	defer span.End()

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
			notFoundErr := errors.New("post not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, err
	}

	// Check permissions
	// Only owner (within 7 days) or admin can restore
	if !isAdmin && post.UserID != userID {
		unauthorizedErr := errors.New("unauthorized")
		recordSpanError(span, unauthorizedErr)
		return nil, unauthorizedErr
	}

	if !isAdmin && post.DeletedAt != nil {
		// Check if within 7 days
		sevenDaysAgo := time.Now().AddDate(0, 0, -7)
		if post.DeletedAt.Before(sevenDaysAgo) {
			permanentErr := errors.New("post permanently deleted")
			recordSpanError(span, permanentErr)
			return nil, permanentErr
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
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to restore post: %w", err)
	}

	post.User = &user

	// Fetch links for this post
	links, err := s.getPostLinks(ctx, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	post.Links = links

	// Fetch images for this post
	images, err := s.getPostImages(ctx, postID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	post.Images = images

	// Fetch reactions
	counts, viewerReactions, err := s.getPostReactions(ctx, postID, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	post.ReactionCounts = counts
	post.ViewerReactions = viewerReactions
	observability.RecordPostRestored(ctx)

	return &post, nil
}

// GetPostsByUserID retrieves a paginated list of posts by a specific user using cursor-based pagination
func (s *PostService) GetPostsByUserID(ctx context.Context, targetUserID uuid.UUID, cursor *string, limit int, viewerID uuid.UUID) (*models.FeedResponse, error) {
	ctx, span := otel.Tracer("clubhouse.posts").Start(ctx, "PostService.GetPostsByUserID")
	span.SetAttributes(
		attribute.String("target_user_id", targetUserID.String()),
		attribute.String("viewer_id", viewerID.String()),
		attribute.Int("limit", limit),
		attribute.Bool("has_cursor", cursor != nil && *cursor != ""),
	)
	defer span.End()

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
		recordSpanError(span, err)
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
			recordSpanError(span, err)
			return nil, err
		}

		post.User = &user

		// Fetch links for this post
		links, err := s.getPostLinks(ctx, post.ID)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		post.Links = links

		// Fetch images for this post
		images, err := s.getPostImages(ctx, post.ID)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		post.Images = images

		// Fetch reactions
		counts, viewerReactions, err := s.getPostReactions(ctx, post.ID, viewerID)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		post.ReactionCounts = counts
		post.ViewerReactions = viewerReactions

		posts = append(posts, &post)
	}

	if err = rows.Err(); err != nil {
		recordSpanError(span, err)
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
	ctx, span := otel.Tracer("clubhouse.posts").Start(ctx, "PostService.HardDeletePost")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.String("admin_user_id", adminUserID.String()),
	)
	defer span.End()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Verify post exists (include soft-deleted posts)
	var exists bool
	err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1)", postID).Scan(&exists)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to check post existence: %w", err)
	}
	if !exists {
		recordSpanError(span, ErrPostNotFound)
		return ErrPostNotFound
	}

	// Create audit log entry BEFORE deleting the post (FK constraint)
	auditQuery := `
		INSERT INTO audit_logs (admin_user_id, action, related_post_id, created_at)
		VALUES ($1, 'hard_delete_post', $2, now())
	`
	_, err = tx.ExecContext(ctx, auditQuery, adminUserID, postID)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	// Delete links associated with comments on this post
	_, err = tx.ExecContext(ctx, "DELETE FROM links WHERE comment_id IN (SELECT id FROM comments WHERE post_id = $1)", postID)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to delete comment links: %w", err)
	}

	// Delete reactions on comments of this post
	_, err = tx.ExecContext(ctx, "DELETE FROM reactions WHERE comment_id IN (SELECT id FROM comments WHERE post_id = $1)", postID)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to delete comment reactions: %w", err)
	}

	// Delete mentions from comments on this post
	_, err = tx.ExecContext(ctx, "DELETE FROM mentions WHERE comment_id IN (SELECT id FROM comments WHERE post_id = $1)", postID)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to delete comment mentions: %w", err)
	}

	// Delete notifications related to this post or its comments
	_, err = tx.ExecContext(ctx, "DELETE FROM notifications WHERE related_post_id = $1 OR related_comment_id IN (SELECT id FROM comments WHERE post_id = $1)", postID)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to delete notifications: %w", err)
	}

	// Delete comments on this post
	_, err = tx.ExecContext(ctx, "DELETE FROM comments WHERE post_id = $1", postID)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to delete comments: %w", err)
	}

	// Delete reactions on this post
	_, err = tx.ExecContext(ctx, "DELETE FROM reactions WHERE post_id = $1", postID)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to delete post reactions: %w", err)
	}

	// Delete mentions from this post
	_, err = tx.ExecContext(ctx, "DELETE FROM mentions WHERE post_id = $1", postID)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to delete post mentions: %w", err)
	}

	// Delete links associated with this post
	_, err = tx.ExecContext(ctx, "DELETE FROM links WHERE post_id = $1", postID)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to delete post links: %w", err)
	}

	// Delete the post
	result, err := tx.ExecContext(ctx, "DELETE FROM posts WHERE id = $1", postID)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to delete post: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		recordSpanError(span, ErrPostNotFound)
		return ErrPostNotFound
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	observability.RecordPostDeleted(ctx)

	return nil
}

// AdminRestorePost restores a soft-deleted post (admin only) with audit logging
func (s *PostService) AdminRestorePost(ctx context.Context, postID uuid.UUID, adminUserID uuid.UUID) (*models.Post, error) {
	ctx, span := otel.Tracer("clubhouse.posts").Start(ctx, "PostService.AdminRestorePost")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.String("admin_user_id", adminUserID.String()),
	)
	defer span.End()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
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
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to check post: %w", err)
	}
	if !exists {
		recordSpanError(span, ErrPostNotFound)
		return nil, ErrPostNotFound
	}
	if !isDeleted {
		notDeletedErr := errors.New("post is not deleted")
		recordSpanError(span, notDeletedErr)
		return nil, notDeletedErr
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
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to restore post: %w", err)
	}

	// Create audit log entry
	auditService := NewAuditService(tx)
	metadata := map[string]interface{}{
		"post_id":             post.ID.String(),
		"section_id":          post.SectionID.String(),
		"restored_by_user_id": adminUserID.String(),
		"restored_by_admin":   true,
		"post_owner_user_id":  post.UserID.String(),
	}
	if err := auditService.LogModerationAudit(
		ctx,
		"restore_post",
		adminUserID,
		post.UserID,
		post.ID,
		uuid.Nil,
		metadata,
	); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Fetch the full post with user info
	fullPost, err := s.GetPostByID(ctx, postID, adminUserID)
	if err != nil {
		recordSpanError(span, err)
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
	if trimmedContent == "" && len(req.Links) == 0 && len(req.Images) == 0 {
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

	if len(req.Images) > maxPostImages {
		return fmt.Errorf("too many images")
	}

	for _, image := range req.Images {
		if strings.TrimSpace(image.URL) == "" {
			return fmt.Errorf("image url cannot be empty")
		}
		if len(image.URL) > 2048 {
			return fmt.Errorf("image url must be less than 2048 characters")
		}
	}

	return nil
}

// validateUpdatePostInput validates post update input
func validateUpdatePostInput(req *models.UpdatePostRequest) error {
	if req == nil {
		return fmt.Errorf("content is required")
	}

	trimmedContent := strings.TrimSpace(req.Content)
	if trimmedContent == "" {
		return fmt.Errorf("content is required")
	}

	if len(trimmedContent) > 5000 {
		return fmt.Errorf("content must be less than 5000 characters")
	}

	if req.Links != nil {
		for _, link := range *req.Links {
			if strings.TrimSpace(link.URL) == "" {
				return fmt.Errorf("link url cannot be empty")
			}
			if len(link.URL) > 2048 {
				return fmt.Errorf("link url must be less than 2048 characters")
			}
		}
	}

	if req.Images != nil {
		if len(*req.Images) > maxPostImages {
			return fmt.Errorf("too many images")
		}
		for _, image := range *req.Images {
			if strings.TrimSpace(image.URL) == "" {
				return fmt.Errorf("image url cannot be empty")
			}
			if len(image.URL) > 2048 {
				return fmt.Errorf("image url must be less than 2048 characters")
			}
		}
	}

	return nil
}

func imageCount(images *[]models.PostImageRequest) int {
	if images == nil {
		return 0
	}
	return len(*images)
}

func normalizePostImageRequests(images []models.PostImageRequest) []models.PostImageRequest {
	normalized := make([]models.PostImageRequest, 0, len(images))
	for _, image := range images {
		normalized = append(normalized, normalizePostImageRequest(image))
	}
	return normalized
}

func normalizePostImageRequest(image models.PostImageRequest) models.PostImageRequest {
	image.URL = strings.TrimSpace(image.URL)
	image.Caption = normalizeOptionalText(image.Caption)
	image.AltText = normalizeOptionalText(image.AltText)
	return image
}

func normalizeOptionalText(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
