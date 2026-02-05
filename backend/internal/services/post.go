package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	linkmeta "github.com/sanderginn/clubhouse/internal/services/links"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// PostService handles post-related operations
type PostService struct {
	db    *sql.DB
	redis *redis.Client
}

const maxPostImages = 10

var imageLinkPattern = regexp.MustCompile(`(?i)\.(jpg|jpeg|png|gif|webp|bmp|svg|avif|tif|tiff)(?:$|[?#&])`)

// NewPostService creates a new post service
func NewPostService(db *sql.DB) *PostService {
	return &PostService{db: db}
}

func NewPostServiceWithRedis(db *sql.DB, rdb *redis.Client) *PostService {
	return &PostService{db: db, redis: rdb}
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
	highlightCount := countLinkHighlights(req.Links)
	if highlightCount > 0 {
		span.SetAttributes(attribute.Int("highlight_count", highlightCount))
		observability.LogDebug(ctx, "post highlights provided", "highlight_count", strconv.Itoa(highlightCount), "section_type", sectionType)
	}

	// Create post ID
	postID := uuid.New()
	trimmedContent := strings.TrimSpace(req.Content)
	shouldEnqueueMetadataJobs := s.redis != nil && GetConfigService().IsLinkMetadataEnabled()
	jobs := make([]MetadataJob, 0, len(req.Links))

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

		for _, linkReq := range req.Links {
			linkID := uuid.New()

			mergedMetadata, sortedHighlights := mergeHighlightsIntoMetadata(linkReq, nil)
			metadataValue := interface{}(nil)
			if len(mergedMetadata) > 0 {
				metadataValue = mergedMetadata
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
				link.Metadata = stripHighlightsFromMetadata(meta)
			}
			if len(sortedHighlights) > 0 {
				link.Highlights = sortedHighlights
			}

			post.Links = append(post.Links, link)

			if shouldEnqueueMetadataJobs && !linkmeta.IsInternalUploadURL(linkReq.URL) {
				jobs = append(jobs, MetadataJob{
					PostID:    post.ID,
					LinkID:    linkID,
					URL:       linkReq.URL,
					CreatedAt: time.Now(),
				})
			}
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

	for _, job := range jobs {
		if err := EnqueueMetadataJob(ctx, s.redis, job); err != nil {
			observability.LogWarn(ctx, "failed to enqueue metadata job",
				"post_id", job.PostID.String(),
				"link_id", job.LinkID.String(),
				"link_url", job.URL,
				"error", err.Error(),
			)
		}
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
		attribute.Bool("remove_link_metadata", req.RemoveLinkMetadata),
	)

	trimmedContent := strings.TrimSpace(req.Content)
	var linkMetadata []models.JSONMap
	linksChanged := false
	linkMetadataRemoved := false
	imagesChanged := false
	var normalizedImages []models.PostImageRequest
	var existingLinks []models.Link
	var removedLink *models.Link

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

	if req.Links != nil || req.RemoveLinkMetadata {
		var err error
		existingLinks, err = s.getPostLinks(ctx, postID, uuid.Nil)
		if err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to fetch post links: %w", err)
		}
	}

	if req.RemoveLinkMetadata {
		removedLink = findPrimaryNonImageLink(existingLinks)
		if removedLink != nil {
			linkMetadataRemoved = true
			if req.Links == nil {
				linksChanged = true
			}
		}
	}

	if req.Links != nil {
		highlightCount := countLinkHighlights(*req.Links)
		if highlightCount > 0 {
			span.SetAttributes(attribute.Int("highlight_count", highlightCount))
			observability.LogDebug(ctx, "post highlights updated", "highlight_count", strconv.Itoa(highlightCount), "section_type", sectionType)
		}

		for _, link := range *req.Links {
			if err := models.ValidateHighlights(sectionType, link.Highlights); err != nil {
				recordSpanError(span, err)
				return nil, err
			}
		}

		linksChanged = !linkRequestsMatchExistingLinks(existingLinks, *req.Links)
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

				var fetchedMetadata models.JSONMap
				if len(linkMetadata) > i && len(linkMetadata[i]) > 0 {
					fetchedMetadata = linkMetadata[i]
				}

				mergedMetadata, _ := mergeHighlightsIntoMetadata(linkReq, fetchedMetadata)
				metadataValue := interface{}(nil)
				if len(mergedMetadata) > 0 {
					metadataValue = mergedMetadata
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

	if req.Links == nil && linkMetadataRemoved && removedLink != nil {
		if _, err := tx.ExecContext(ctx, "DELETE FROM links WHERE id = $1", removedLink.ID); err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to delete link metadata: %w", err)
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
		"post_id":               postID.String(),
		"section_id":            sectionID.String(),
		"content_excerpt":       truncateAuditExcerpt(trimmedContent),
		"previous_content":      previousContent,
		"links_changed":         linksChanged,
		"links_provided":        req.Links != nil,
		"link_metadata_removed": linkMetadataRemoved,
		"images_changed":        imagesChanged,
		"images_provided":       req.Images != nil,
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
	if linkMetadataRemoved && removedLink != nil {
		removalMetadata := map[string]interface{}{
			"post_id":    postID.String(),
			"section_id": sectionID.String(),
			"link_id":    removedLink.ID.String(),
			"link_url":   removedLink.URL,
		}
		if err := auditService.LogAuditWithMetadata(ctx, "remove_link_metadata", userID, ownerID, removalMetadata); err != nil {
			recordSpanError(span, err)
			return nil, fmt.Errorf("failed to create link metadata removal audit log: %w", err)
		}
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
			COALESCE(COUNT(DISTINCT c.id), 0) as comment_count,
			s.type
		FROM posts p
		JOIN users u ON p.user_id = u.id
		JOIN sections s ON p.section_id = s.id
		LEFT JOIN comments c ON p.id = c.post_id AND c.deleted_at IS NULL
		WHERE p.id = $1 AND p.deleted_at IS NULL
		GROUP BY p.id, u.id, s.type
	`

	var post models.Post
	var user models.User
	var sectionType string

	err := s.db.QueryRowContext(ctx, query, postID).Scan(
		&post.ID, &post.UserID, &post.SectionID, &post.Content,
		&post.CreatedAt, &post.UpdatedAt, &post.DeletedAt, &post.DeletedByUserID,
		&user.ID, &user.Username, &user.Email, &user.ProfilePictureURL, &user.Bio, &user.IsAdmin, &user.CreatedAt,
		&post.CommentCount, &sectionType,
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
	links, err := s.getPostLinks(ctx, postID, userID)
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

	if sectionType == "recipe" {
		viewerID := &userID
		if userID == uuid.Nil {
			viewerID = nil
		}
		recipeStats, err := s.getRecipeStats(ctx, postID, viewerID)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		post.RecipeStats = recipeStats
	}

	return &post, nil
}

// getPostLinks retrieves all links for a post
func (s *PostService) getPostLinks(ctx context.Context, postID uuid.UUID, viewerID uuid.UUID) ([]models.Link, error) {
	ctx, span := otel.Tracer("clubhouse.posts").Start(ctx, "PostService.getPostLinks")
	span.SetAttributes(attribute.String("post_id", postID.String()))
	defer span.End()

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
	highlightCount := 0
	for rows.Next() {
		var link models.Link
		var metadataJSON sql.NullString

		err := rows.Scan(&link.ID, &link.URL, &metadataJSON, &link.CreatedAt)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}

		// Parse metadata if present
		if metadataJSON.Valid {
			var metadata map[string]interface{}
			if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err != nil {
				observability.LogWarn(ctx, "failed to parse link metadata", "post_id", postID.String(), "link_id", link.ID.String())
			} else {
				highlights, err := extractHighlightsFromMetadata(metadata)
				if err != nil {
					observability.LogWarn(ctx, "failed to parse link highlights", "post_id", postID.String(), "link_id", link.ID.String())
				} else if len(highlights) > 0 {
					link.Highlights = highlights
					highlightCount += len(highlights)
					delete(metadata, "highlights")
				}
				if len(metadata) > 0 {
					link.Metadata = metadata
				}
			}
		}

		links = append(links, link)
	}

	if err = rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if highlightCount > 0 {
		if err := s.populateHighlightReactions(ctx, links, viewerID); err != nil {
			recordSpanError(span, err)
			return nil, err
		}
	}

	span.SetAttributes(
		attribute.Int("link_count", len(links)),
		attribute.Int("highlight_count", highlightCount),
	)
	return links, nil
}

func (s *PostService) populateHighlightReactions(ctx context.Context, links []models.Link, viewerID uuid.UUID) error {
	if len(links) == 0 {
		return nil
	}
	ctx, span := otel.Tracer("clubhouse.posts").Start(ctx, "PostService.populateHighlightReactions")
	defer span.End()

	linkIDs := make([]uuid.UUID, 0, len(links))
	highlightTotal := 0
	for i := range links {
		if len(links[i].Highlights) == 0 {
			continue
		}
		linkIDs = append(linkIDs, links[i].ID)
		for j := range links[i].Highlights {
			highlightTotal++
			highlightID, err := models.EncodeHighlightID(links[i].ID, links[i].Highlights[j])
			if err != nil {
				observability.LogWarn(ctx, "failed to encode highlight id", "link_id", links[i].ID.String())
				continue
			}
			links[i].Highlights[j].ID = highlightID
		}
	}

	if len(linkIDs) == 0 {
		return nil
	}

	counts := make(map[string]int)
	rows, err := s.db.QueryContext(ctx, `
		SELECT highlight_id, COUNT(*)
		FROM highlight_reactions
		WHERE link_id = ANY($1)
		GROUP BY highlight_id
	`, pq.Array(linkIDs))
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	for rows.Next() {
		var highlightID string
		var count int
		if err := rows.Scan(&highlightID, &count); err != nil {
			_ = rows.Close()
			recordSpanError(span, err)
			return err
		}
		counts[highlightID] = count
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		recordSpanError(span, err)
		return err
	}
	_ = rows.Close()

	viewerReactions := make(map[string]struct{})
	if viewerID != uuid.Nil {
		viewerRows, err := s.db.QueryContext(ctx, `
			SELECT highlight_id
			FROM highlight_reactions
			WHERE user_id = $1 AND link_id = ANY($2)
		`, viewerID, pq.Array(linkIDs))
		if err != nil {
			recordSpanError(span, err)
			return err
		}
		for viewerRows.Next() {
			var highlightID string
			if err := viewerRows.Scan(&highlightID); err != nil {
				_ = viewerRows.Close()
				recordSpanError(span, err)
				return err
			}
			viewerReactions[highlightID] = struct{}{}
		}
		if err := viewerRows.Err(); err != nil {
			_ = viewerRows.Close()
			recordSpanError(span, err)
			return err
		}
		_ = viewerRows.Close()
	}

	for i := range links {
		for j := range links[i].Highlights {
			highlightID := links[i].Highlights[j].ID
			if highlightID == "" {
				continue
			}
			if count, ok := counts[highlightID]; ok {
				links[i].Highlights[j].HeartCount = count
			}
			if _, ok := viewerReactions[highlightID]; ok {
				links[i].Highlights[j].ViewerReacted = true
			}
		}
	}

	span.SetAttributes(attribute.Int("highlight_count", highlightTotal))
	return nil
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

func countLinkHighlights(links []models.LinkRequest) int {
	count := 0
	for _, link := range links {
		count += len(link.Highlights)
	}
	return count
}

func mergeHighlightsIntoMetadata(link models.LinkRequest, fetched models.JSONMap) (models.JSONMap, []models.Highlight) {
	sortedHighlights := sortHighlights(sanitizeHighlights(link.Highlights))
	if len(sortedHighlights) == 0 && len(fetched) == 0 {
		return nil, sortedHighlights
	}
	metadata := make(models.JSONMap)
	for key, value := range fetched {
		metadata[key] = value
	}
	if len(sortedHighlights) > 0 {
		metadata["highlights"] = sortedHighlights
	}
	return metadata, sortedHighlights
}

func stripHighlightsFromMetadata(metadata models.JSONMap) map[string]interface{} {
	if len(metadata) == 0 {
		return nil
	}
	trimmed := make(map[string]interface{}, len(metadata))
	for key, value := range metadata {
		if key == "highlights" {
			continue
		}
		trimmed[key] = value
	}
	if len(trimmed) == 0 {
		return nil
	}
	return trimmed
}

func extractHighlightsFromMetadata(metadata map[string]interface{}) ([]models.Highlight, error) {
	raw, ok := metadata["highlights"]
	if !ok {
		return nil, nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var highlights []models.Highlight
	if err := json.Unmarshal(encoded, &highlights); err != nil {
		return nil, err
	}
	return sortHighlights(highlights), nil
}

func sanitizeHighlights(highlights []models.Highlight) []models.Highlight {
	if len(highlights) == 0 {
		return nil
	}
	sanitized := make([]models.Highlight, 0, len(highlights))
	for _, highlight := range highlights {
		sanitized = append(sanitized, models.Highlight{
			Timestamp: highlight.Timestamp,
			Label:     highlight.Label,
		})
	}
	return sanitized
}

func sortHighlights(highlights []models.Highlight) []models.Highlight {
	if len(highlights) == 0 {
		return nil
	}
	sorted := append([]models.Highlight(nil), highlights...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Timestamp < sorted[j].Timestamp
	})
	return sorted
}

func linkRequestsMatchExistingLinks(existing []models.Link, requested []models.LinkRequest) bool {
	if len(existing) != len(requested) {
		return false
	}
	for i, link := range requested {
		if existing[i].URL != link.URL {
			return false
		}
		existingHighlights := sortHighlights(sanitizeHighlights(existing[i].Highlights))
		requestedHighlights := sortHighlights(sanitizeHighlights(link.Highlights))
		if len(existingHighlights) != len(requestedHighlights) {
			return false
		}
		for j := range existingHighlights {
			if existingHighlights[j] != requestedHighlights[j] {
				return false
			}
		}
	}
	return true
}

func findPrimaryNonImageLink(links []models.Link) *models.Link {
	for i := range links {
		if !isImageLink(links[i]) {
			return &links[i]
		}
	}
	return nil
}

func isImageLink(link models.Link) bool {
	if link.URL == "" {
		return false
	}
	if link.Metadata != nil {
		if rawType, ok := link.Metadata["type"].(string); ok {
			normalized := strings.ToLower(rawType)
			if normalized == "image" || strings.HasPrefix(normalized, "image/") {
				return true
			}
		}
	}
	return imageLinkPattern.MatchString(link.URL)
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

func (s *PostService) getRecipeStats(ctx context.Context, postID uuid.UUID, viewerID *uuid.UUID) (*models.RecipeStats, error) {
	statsByPost, err := s.getRecipeStatsForPosts(ctx, []uuid.UUID{postID}, viewerID)
	if err != nil {
		return nil, err
	}
	stats, ok := statsByPost[postID]
	if !ok {
		return &models.RecipeStats{}, nil
	}
	return stats, nil
}

func (s *PostService) getRecipeStatsForPosts(ctx context.Context, postIDs []uuid.UUID, viewerID *uuid.UUID) (map[uuid.UUID]*models.RecipeStats, error) {
	ctx, span := otel.Tracer("clubhouse.posts").Start(ctx, "PostService.getRecipeStatsForPosts")
	span.SetAttributes(
		attribute.Int("post_count", len(postIDs)),
		attribute.Bool("has_viewer_id", viewerID != nil),
	)
	if viewerID != nil {
		span.SetAttributes(attribute.String("viewer_id", viewerID.String()))
	}
	defer span.End()

	stats := make(map[uuid.UUID]*models.RecipeStats, len(postIDs))
	for _, postID := range postIDs {
		stats[postID] = &models.RecipeStats{}
	}

	if len(postIDs) == 0 {
		return stats, nil
	}

	viewerIDValue := uuid.Nil
	if viewerID != nil {
		viewerIDValue = *viewerID
	}

	saveRows, err := s.db.QueryContext(ctx, `
		SELECT sr.post_id, COUNT(DISTINCT sr.id) AS save_count, bool_or(sr.user_id = $2) AS viewer_saved
		FROM saved_recipes sr
		WHERE sr.post_id = ANY($1) AND sr.deleted_at IS NULL
		GROUP BY sr.post_id
	`, pq.Array(postIDs), viewerIDValue)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	for saveRows.Next() {
		var postID uuid.UUID
		var saveCount int
		var viewerSaved bool
		if err := saveRows.Scan(&postID, &saveCount, &viewerSaved); err != nil {
			_ = saveRows.Close()
			recordSpanError(span, err)
			return nil, err
		}
		if stat, ok := stats[postID]; ok {
			stat.SaveCount = saveCount
			stat.ViewerSaved = viewerSaved
		}
	}
	if err := saveRows.Err(); err != nil {
		_ = saveRows.Close()
		recordSpanError(span, err)
		return nil, err
	}
	_ = saveRows.Close()

	cookRows, err := s.db.QueryContext(ctx, `
		SELECT cl.post_id, COUNT(*) AS cook_count, ROUND(AVG(cl.rating)::numeric, 1) AS avg_rating, bool_or(cl.user_id = $2) AS viewer_cooked
		FROM cook_logs cl
		WHERE cl.post_id = ANY($1) AND cl.deleted_at IS NULL
		GROUP BY cl.post_id
	`, pq.Array(postIDs), viewerIDValue)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	for cookRows.Next() {
		var postID uuid.UUID
		var cookCount int
		var avgRating sql.NullFloat64
		var viewerCooked bool
		if err := cookRows.Scan(&postID, &cookCount, &avgRating, &viewerCooked); err != nil {
			_ = cookRows.Close()
			recordSpanError(span, err)
			return nil, err
		}
		if stat, ok := stats[postID]; ok {
			stat.CookCount = cookCount
			stat.ViewerCooked = viewerCooked
			if avgRating.Valid {
				stat.AvgRating = &avgRating.Float64
			}
		}
	}
	if err := cookRows.Err(); err != nil {
		_ = cookRows.Close()
		recordSpanError(span, err)
		return nil, err
	}
	_ = cookRows.Close()

	if viewerID != nil {
		categoryRows, err := s.db.QueryContext(ctx, `
			SELECT post_id, category
			FROM saved_recipes
			WHERE post_id = ANY($1) AND user_id = $2 AND deleted_at IS NULL
			ORDER BY category ASC
		`, pq.Array(postIDs), *viewerID)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		for categoryRows.Next() {
			var postID uuid.UUID
			var category string
			if err := categoryRows.Scan(&postID, &category); err != nil {
				_ = categoryRows.Close()
				recordSpanError(span, err)
				return nil, err
			}
			if stat, ok := stats[postID]; ok {
				stat.ViewerCategories = append(stat.ViewerCategories, category)
			}
		}
		if err := categoryRows.Err(); err != nil {
			_ = categoryRows.Close()
			recordSpanError(span, err)
			return nil, err
		}
		_ = categoryRows.Close()
	}

	return stats, nil
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

	var sectionType string
	if err := s.db.QueryRowContext(ctx, "SELECT type FROM sections WHERE id = $1", sectionID).Scan(&sectionType); err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	span.SetAttributes(attribute.String("section_type", sectionType))

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
		links, err := s.getPostLinks(ctx, post.ID, userID)
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

	if sectionType == "recipe" && len(posts) > 0 {
		postIDs := make([]uuid.UUID, 0, len(posts))
		for _, post := range posts {
			postIDs = append(postIDs, post.ID)
		}
		viewerID := &userID
		if userID == uuid.Nil {
			viewerID = nil
		}
		statsByPost, err := s.getRecipeStatsForPosts(ctx, postIDs, viewerID)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		for _, post := range posts {
			if stat, ok := statsByPost[post.ID]; ok {
				post.RecipeStats = stat
			}
		}
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
	links, err := s.getPostLinks(ctx, postID, userID)
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
			COALESCE(COUNT(DISTINCT c.id), 0) as comment_count,
			s.type
		FROM posts p
		JOIN users u ON p.user_id = u.id
		JOIN sections s ON p.section_id = s.id
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

	query += fmt.Sprintf(" GROUP BY p.id, u.id, s.type ORDER BY p.created_at DESC LIMIT $%d", argIndex)
	args = append(args, limit+1) // Fetch one extra to determine if hasMore

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	defer rows.Close()

	var posts []*models.Post
	var recipePostIDs []uuid.UUID
	for rows.Next() {
		var post models.Post
		var user models.User
		var sectionType string

		err := rows.Scan(
			&post.ID, &post.UserID, &post.SectionID, &post.Content,
			&post.CreatedAt, &post.UpdatedAt, &post.DeletedAt, &post.DeletedByUserID,
			&user.ID, &user.Username, &user.Email, &user.ProfilePictureURL, &user.Bio, &user.IsAdmin, &user.CreatedAt,
			&post.CommentCount, &sectionType,
		)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}

		post.User = &user

		// Fetch links for this post
		links, err := s.getPostLinks(ctx, post.ID, viewerID)
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

		if sectionType == "recipe" {
			recipePostIDs = append(recipePostIDs, post.ID)
		}

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

	if len(recipePostIDs) > 0 {
		viewerIDPtr := &viewerID
		if viewerID == uuid.Nil {
			viewerIDPtr = nil
		}
		statsByPost, err := s.getRecipeStatsForPosts(ctx, recipePostIDs, viewerIDPtr)
		if err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		for _, post := range posts {
			if stat, ok := statsByPost[post.ID]; ok {
				post.RecipeStats = stat
			}
		}
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
