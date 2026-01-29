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

// CommentService handles comment-related operations
type CommentService struct {
	db *sql.DB
}

// NewCommentService creates a new comment service
func NewCommentService(db *sql.DB) *CommentService {
	return &CommentService{db: db}
}

// CreateComment creates a new comment with optional links
func (s *CommentService) CreateComment(ctx context.Context, req *models.CreateCommentRequest, userID uuid.UUID) (*models.Comment, error) {
	ctx, span := otel.Tracer("clubhouse.comments").Start(ctx, "CommentService.CreateComment")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.Int("content_length", len(strings.TrimSpace(req.Content))),
		attribute.Bool("has_links", len(req.Links) > 0),
	)
	defer span.End()

	// Validate input
	if err := validateCreateCommentInput(req); err != nil {
		return nil, err
	}

	// Parse and validate post ID
	postID, err := uuid.Parse(req.PostID)
	if err != nil {
		return nil, fmt.Errorf("invalid post id")
	}
	span.SetAttributes(attribute.String("post_id", postID.String()))

	// Verify post exists and is not deleted
	var postExists bool
	err = s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1 AND deleted_at IS NULL)", postID).
		Scan(&postExists)
	if err != nil || !postExists {
		return nil, fmt.Errorf("post not found")
	}

	// Validate parent comment if provided
	var parentCommentID *uuid.UUID
	if req.ParentCommentID != nil {
		parsedParentID, err := uuid.Parse(*req.ParentCommentID)
		if err != nil {
			return nil, fmt.Errorf("invalid parent comment id")
		}

		// Verify parent comment exists, is not deleted, and belongs to the same post
		var parentExists bool
		err = s.db.QueryRowContext(
			ctx,
			"SELECT EXISTS(SELECT 1 FROM comments WHERE id = $1 AND post_id = $2 AND deleted_at IS NULL)",
			parsedParentID,
			postID,
		).Scan(&parentExists)
		if err != nil || !parentExists {
			return nil, fmt.Errorf("parent comment not found")
		}

		parentCommentID = &parsedParentID
	}

	// Create comment ID
	commentID := uuid.New()

	linkMetadata := fetchLinkMetadata(ctx, req.Links)

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Insert comment
	query := `
		INSERT INTO comments (id, user_id, post_id, parent_comment_id, content, created_at)
		VALUES ($1, $2, $3, $4, $5, now())
		RETURNING id, user_id, post_id, parent_comment_id, content, created_at
	`

	var comment models.Comment
	err = tx.QueryRowContext(ctx, query, commentID, userID, postID, parentCommentID, req.Content).
		Scan(&comment.ID, &comment.UserID, &comment.PostID, &comment.ParentCommentID, &comment.Content, &comment.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	// Insert links if provided
	if len(req.Links) > 0 {
		comment.Links = make([]models.Link, 0, len(req.Links))

		for i, linkReq := range req.Links {
			linkID := uuid.New()

			metadataValue := interface{}(nil)
			if len(linkMetadata) > i && len(linkMetadata[i]) > 0 {
				metadataValue = linkMetadata[i]
			}

			// Insert link for comment
			linkQuery := `
				INSERT INTO links (id, comment_id, url, metadata, created_at)
				VALUES ($1, $2, $3, $4, now())
				RETURNING id, url, created_at
			`

			var link models.Link
			err := tx.QueryRowContext(ctx, linkQuery, linkID, commentID, linkReq.URL, metadataValue).
				Scan(&link.ID, &link.URL, &link.CreatedAt)

			if err != nil {
				return nil, fmt.Errorf("failed to create link: %w", err)
			}

			if meta, ok := metadataValue.(models.JSONMap); ok && len(meta) > 0 {
				link.Metadata = map[string]interface{}(meta)
			}

			comment.Links = append(comment.Links, link)
		}
	}

	var user models.User
	err = tx.QueryRowContext(ctx, `
		SELECT id, username, COALESCE(email, '') as email, profile_picture_url, bio, is_admin, created_at
		FROM users WHERE id = $1
	`, userID).Scan(
		&user.ID, &user.Username, &user.Email, &user.ProfilePictureURL, &user.Bio, &user.IsAdmin, &user.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch comment user: %w", err)
	}
	comment.User = &user

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	observability.RecordCommentCreated(ctx)
	return &comment, nil
}

// UpdateComment updates a comment's content and links (author only).

func (s *CommentService) UpdateComment(ctx context.Context, commentID uuid.UUID, userID uuid.UUID, req *models.UpdateCommentRequest) (*models.Comment, error) {
	ctx, span := otel.Tracer("clubhouse.comments").Start(ctx, "CommentService.UpdateComment")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("comment_id", commentID.String()),
		attribute.Int("content_length", len(strings.TrimSpace(req.Content))),
		attribute.Bool("has_links", req.Links != nil && len(*req.Links) > 0),
	)
	defer span.End()

	if err := validateUpdateCommentInput(req); err != nil {
		return nil, err
	}

	trimmedContent := strings.TrimSpace(req.Content)
	var linkMetadata []models.JSONMap
	linksChanged := false

	var ownerID uuid.UUID
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id
		FROM comments
		WHERE id = $1 AND deleted_at IS NULL
	`, commentID).Scan(&ownerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("comment not found")
		}
		return nil, fmt.Errorf("failed to fetch comment owner: %w", err)
	}

	if ownerID != userID {
		return nil, errors.New("unauthorized to edit this comment")
	}

	if req.Links != nil {
		existingURLs, err := getCommentLinkURLs(ctx, s.db, commentID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch comment links: %w", err)
		}

		linksChanged = !linkRequestsMatchURLs(existingURLs, *req.Links)
		if linksChanged && len(*req.Links) > 0 {
			linkMetadata = fetchLinkMetadata(ctx, *req.Links)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.ExecContext(ctx, `
		UPDATE comments
		SET content = $1, updated_at = now()
		WHERE id = $2
	`, trimmedContent, commentID)
	if err != nil {
		return nil, fmt.Errorf("failed to update comment: %w", err)
	}

	if req.Links != nil && linksChanged {
		if _, err := tx.ExecContext(ctx, "DELETE FROM links WHERE comment_id = $1", commentID); err != nil {
			return nil, fmt.Errorf("failed to delete comment links: %w", err)
		}

		if len(*req.Links) > 0 {
			for i, linkReq := range *req.Links {
				linkID := uuid.New()

				metadataValue := interface{}(nil)
				if len(linkMetadata) > i && len(linkMetadata[i]) > 0 {
					metadataValue = linkMetadata[i]
				}

				_, err := tx.ExecContext(ctx, `
					INSERT INTO links (id, comment_id, url, metadata, created_at)
					VALUES ($1, $2, $3, $4, now())
				`, linkID, commentID, linkReq.URL, metadataValue)
				if err != nil {
					return nil, fmt.Errorf("failed to create link: %w", err)
				}
			}
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs (admin_user_id, action, related_comment_id, related_user_id, created_at)
		VALUES ($1, 'update_comment', $2, $3, now())
	`, userID, commentID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return s.GetCommentByID(ctx, commentID, userID)
}

// GetCommentByID retrieves a comment by ID with all related data
func (s *CommentService) GetCommentByID(ctx context.Context, commentID uuid.UUID, userID uuid.UUID) (*models.Comment, error) {
	query := `
		SELECT
			c.id, c.user_id, c.post_id, p.section_id, c.parent_comment_id, c.content,
			c.created_at, c.updated_at, c.deleted_at, c.deleted_by_user_id,
			u.id, u.username, COALESCE(u.email, '') as email, u.profile_picture_url, u.bio, u.is_admin, u.created_at
		FROM comments c
		JOIN posts p ON c.post_id = p.id
		JOIN users u ON c.user_id = u.id
		WHERE c.id = $1 AND c.deleted_at IS NULL
	`

	var comment models.Comment
	var user models.User

	var sectionID uuid.UUID
	err := s.db.QueryRowContext(ctx, query, commentID).Scan(
		&comment.ID, &comment.UserID, &comment.PostID, &sectionID, &comment.ParentCommentID, &comment.Content,
		&comment.CreatedAt, &comment.UpdatedAt, &comment.DeletedAt, &comment.DeletedByUserID,
		&user.ID, &user.Username, &user.Email, &user.ProfilePictureURL, &user.Bio, &user.IsAdmin, &user.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("comment not found")
		}
		return nil, err
	}

	comment.User = &user
	comment.SectionID = &sectionID

	// Fetch links for this comment
	links, err := s.getCommentLinks(ctx, commentID)
	if err != nil {
		return nil, err
	}
	comment.Links = links

	// Fetch reactions
	counts, viewerReactions, err := s.getCommentReactions(ctx, commentID, userID)
	if err != nil {
		return nil, err
	}
	comment.ReactionCounts = counts
	comment.ViewerReactions = viewerReactions

	return &comment, nil
}

// GetCommentContext retrieves the post and section IDs for a comment.
func (s *CommentService) GetCommentContext(ctx context.Context, commentID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	query := `
		SELECT c.post_id, p.section_id
		FROM comments c
		JOIN posts p ON c.post_id = p.id
		WHERE c.id = $1 AND c.deleted_at IS NULL
	`

	var postID uuid.UUID
	var sectionID uuid.UUID
	if err := s.db.QueryRowContext(ctx, query, commentID).Scan(&postID, &sectionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.UUID{}, uuid.UUID{}, errors.New("comment not found")
		}
		return uuid.UUID{}, uuid.UUID{}, err
	}

	return postID, sectionID, nil
}

// getCommentLinks retrieves all links for a comment
func (s *CommentService) getCommentLinks(ctx context.Context, commentID uuid.UUID) ([]models.Link, error) {
	query := `
		SELECT id, url, metadata, created_at
		FROM links
		WHERE comment_id = $1
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, commentID)
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

func getCommentLinkURLs(ctx context.Context, queryer interface {
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
}, commentID uuid.UUID) ([]string, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT url
		FROM links
		WHERE comment_id = $1
		ORDER BY created_at ASC
	`, commentID)
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

// getCommentReactions retrieves reaction counts and viewer reactions for a comment
func (s *CommentService) getCommentReactions(ctx context.Context, commentID uuid.UUID, viewerID uuid.UUID) (map[string]int, []string, error) {
	// Get counts
	rows, err := s.db.QueryContext(ctx, `
		SELECT emoji, COUNT(*)
		FROM reactions
		WHERE comment_id = $1 AND deleted_at IS NULL
		GROUP BY emoji
	`, commentID)
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
			WHERE comment_id = $1 AND user_id = $2 AND deleted_at IS NULL
		`, commentID, viewerID)
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

// validateCreateCommentInput validates comment creation input
func validateCreateCommentInput(req *models.CreateCommentRequest) error {
	if strings.TrimSpace(req.PostID) == "" {
		return fmt.Errorf("post_id is required")
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

// GetThreadComments retrieves all comments for a post with cursor-based pagination
func (s *CommentService) GetThreadComments(ctx context.Context, postID uuid.UUID, limit int, cursor *string, userID uuid.UUID) ([]models.Comment, *string, bool, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// Validate post exists and is not deleted
	var postExists bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1 AND deleted_at IS NULL)", postID).Scan(&postExists)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to check post existence: %w", err)
	}
	if !postExists {
		return nil, nil, false, errors.New("post not found")
	}

	// Build query for top-level comments
	query := `
		SELECT
			c.id, c.user_id, c.post_id, c.parent_comment_id, c.content,
			c.created_at, c.updated_at, c.deleted_at, c.deleted_by_user_id,
			u.id, u.username, COALESCE(u.email, '') as email, u.profile_picture_url, u.bio, u.is_admin, u.created_at
		FROM comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.post_id = $1 AND c.parent_comment_id IS NULL AND c.deleted_at IS NULL
	`

	args := []interface{}{postID}

	// Apply cursor pagination
	if cursor != nil && *cursor != "" {
		cursorID, err := uuid.Parse(*cursor)
		if err != nil {
			return nil, nil, false, errors.New("invalid cursor")
		}

		// Get cursor comment's creation time
		var cursorTime sql.NullTime
		err = s.db.QueryRowContext(ctx, "SELECT created_at FROM comments WHERE id = $1", cursorID).Scan(&cursorTime)
		if err == sql.ErrNoRows {
			return nil, nil, false, errors.New("cursor not found")
		}
		if err != nil {
			return nil, nil, false, fmt.Errorf("failed to get cursor time: %w", err)
		}

		query += " AND c.created_at < $2 ORDER BY c.created_at DESC LIMIT $3"
		args = append(args, cursorTime.Time, limit+1)
	} else {
		query += " ORDER BY c.created_at DESC LIMIT $2"
		args = append(args, limit+1)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to query comments: %w", err)
	}
	defer rows.Close()

	var comments []models.Comment
	for rows.Next() {
		var c models.Comment
		var user models.User
		var parentID sql.NullString
		var deletedAt sql.NullTime
		var deletedByUserID sql.NullString
		var updatedAt sql.NullTime

		err := rows.Scan(
			&c.ID, &c.UserID, &c.PostID, &parentID, &c.Content,
			&c.CreatedAt, &updatedAt, &deletedAt, &deletedByUserID,
			&user.ID, &user.Username, &user.Email, &user.ProfilePictureURL, &user.Bio, &user.IsAdmin, &user.CreatedAt,
		)
		if err != nil {
			return nil, nil, false, fmt.Errorf("failed to scan comment: %w", err)
		}

		if parentID.Valid {
			pid, _ := uuid.Parse(parentID.String)
			c.ParentCommentID = &pid
		}
		if deletedAt.Valid {
			c.DeletedAt = &deletedAt.Time
		}
		if deletedByUserID.Valid {
			dbuid, _ := uuid.Parse(deletedByUserID.String)
			c.DeletedByUserID = &dbuid
		}
		if updatedAt.Valid {
			c.UpdatedAt = &updatedAt.Time
		}

		c.User = &user

		// Fetch links for this comment
		links, err := s.getCommentLinks(ctx, c.ID)
		if err != nil {
			return nil, nil, false, fmt.Errorf("failed to get comment links: %w", err)
		}
		c.Links = links

		// Fetch reactions
		counts, viewerReactions, err := s.getCommentReactions(ctx, c.ID, userID)
		if err != nil {
			return nil, nil, false, fmt.Errorf("failed to get comment reactions: %w", err)
		}
		c.ReactionCounts = counts
		c.ViewerReactions = viewerReactions

		comments = append(comments, c)
	}

	if err = rows.Err(); err != nil {
		return nil, nil, false, fmt.Errorf("error iterating rows: %w", err)
	}

	// Check if there are more results
	hasMore := false
	var nextCursor *string
	if len(comments) > limit {
		hasMore = true
		comments = comments[:limit]
		nextCursorID := comments[len(comments)-1].ID.String()
		nextCursor = &nextCursorID
	}

	// Fetch replies for each top-level comment
	for i := range comments {
		replies, err := s.getCommentReplies(ctx, comments[i].ID, userID)
		if err != nil {
			return nil, nil, false, fmt.Errorf("failed to get comment replies: %w", err)
		}
		comments[i].Replies = replies
	}

	return comments, nextCursor, hasMore, nil
}

// getCommentReplies retrieves all replies to a comment
func (s *CommentService) getCommentReplies(ctx context.Context, parentCommentID uuid.UUID, userID uuid.UUID) ([]models.Comment, error) {
	query := `
		SELECT
			c.id, c.user_id, c.post_id, c.parent_comment_id, c.content,
			c.created_at, c.updated_at, c.deleted_at, c.deleted_by_user_id,
			u.id, u.username, COALESCE(u.email, '') as email, u.profile_picture_url, u.bio, u.is_admin, u.created_at
		FROM comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.parent_comment_id = $1 AND c.deleted_at IS NULL
		ORDER BY c.created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, parentCommentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query replies: %w", err)
	}
	defer rows.Close()

	var replies []models.Comment
	for rows.Next() {
		var c models.Comment
		var user models.User
		var parentID sql.NullString
		var deletedAt sql.NullTime
		var deletedByUserID sql.NullString
		var updatedAt sql.NullTime

		err := rows.Scan(
			&c.ID, &c.UserID, &c.PostID, &parentID, &c.Content,
			&c.CreatedAt, &updatedAt, &deletedAt, &deletedByUserID,
			&user.ID, &user.Username, &user.Email, &user.ProfilePictureURL, &user.Bio, &user.IsAdmin, &user.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan reply: %w", err)
		}

		if parentID.Valid {
			pid, _ := uuid.Parse(parentID.String)
			c.ParentCommentID = &pid
		}
		if deletedAt.Valid {
			c.DeletedAt = &deletedAt.Time
		}
		if deletedByUserID.Valid {
			dbuid, _ := uuid.Parse(deletedByUserID.String)
			c.DeletedByUserID = &dbuid
		}
		if updatedAt.Valid {
			c.UpdatedAt = &updatedAt.Time
		}

		c.User = &user

		// Fetch links for this reply
		links, err := s.getCommentLinks(ctx, c.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get reply links: %w", err)
		}
		c.Links = links

		// Fetch reactions
		counts, viewerReactions, err := s.getCommentReactions(ctx, c.ID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get reply reactions: %w", err)
		}
		c.ReactionCounts = counts
		c.ViewerReactions = viewerReactions

		replies = append(replies, c)
	}

	return replies, rows.Err()
}

// DeleteComment soft deletes a comment by setting deleted_at and deleted_by_user_id
// Only the comment owner or an admin can delete
// If admin deletes, an audit log entry is created
func (s *CommentService) DeleteComment(ctx context.Context, commentID uuid.UUID, userID uuid.UUID, isAdmin bool) (*models.Comment, error) {
	comment, err := s.GetCommentByID(ctx, commentID, userID)
	if err != nil {
		return nil, err
	}

	if comment.UserID != userID && !isAdmin {
		return nil, errors.New("unauthorized to delete this comment")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	query := `
		UPDATE comments
		SET deleted_at = now(), deleted_by_user_id = $1
		WHERE id = $2
		RETURNING id, user_id, post_id, parent_comment_id, content, created_at, updated_at, deleted_at, deleted_by_user_id
	`

	var updatedComment models.Comment
	var parentID sql.NullString
	var updatedAt sql.NullTime

	err = tx.QueryRowContext(ctx, query, userID, commentID).Scan(
		&updatedComment.ID, &updatedComment.UserID, &updatedComment.PostID, &parentID, &updatedComment.Content,
		&updatedComment.CreatedAt, &updatedAt, &updatedComment.DeletedAt, &updatedComment.DeletedByUserID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to delete comment: %w", err)
	}

	if parentID.Valid {
		pid, _ := uuid.Parse(parentID.String)
		updatedComment.ParentCommentID = &pid
	}
	if updatedAt.Valid {
		updatedComment.UpdatedAt = &updatedAt.Time
	}

	if isAdmin && comment.UserID != userID {
		auditQuery := `
			INSERT INTO audit_logs (admin_user_id, action, related_comment_id, created_at)
			VALUES ($1, 'delete_comment', $2, now())
		`
		_, err = tx.ExecContext(ctx, auditQuery, userID, commentID)
		if err != nil {
			return nil, fmt.Errorf("failed to create audit log: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	updatedComment.User = comment.User
	updatedComment.Links = comment.Links
	updatedComment.ReactionCounts = comment.ReactionCounts
	updatedComment.ViewerReactions = comment.ViewerReactions
	observability.RecordCommentDeleted(ctx)

	return &updatedComment, nil
}

// RestoreComment restores a soft-deleted comment
// Only the comment owner (within 7 days) or an admin can restore
func (s *CommentService) RestoreComment(ctx context.Context, commentID uuid.UUID, userID uuid.UUID, isAdmin bool) (*models.Comment, error) {
	query := `
		SELECT
			c.id, c.user_id, c.post_id, c.parent_comment_id, c.content,
			c.created_at, c.updated_at, c.deleted_at, c.deleted_by_user_id,
			u.id, u.username, COALESCE(u.email, '') as email, u.profile_picture_url, u.bio, u.is_admin, u.created_at
		FROM comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.id = $1 AND c.deleted_at IS NOT NULL
	`

	var comment models.Comment
	var user models.User
	var parentID sql.NullString
	var updatedAt sql.NullTime
	var deletedAt sql.NullTime
	var deletedByUserID sql.NullString

	err := s.db.QueryRowContext(ctx, query, commentID).Scan(
		&comment.ID, &comment.UserID, &comment.PostID, &parentID, &comment.Content,
		&comment.CreatedAt, &updatedAt, &deletedAt, &deletedByUserID,
		&user.ID, &user.Username, &user.Email, &user.ProfilePictureURL, &user.Bio, &user.IsAdmin, &user.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("comment not found")
		}
		return nil, err
	}

	if parentID.Valid {
		pid, _ := uuid.Parse(parentID.String)
		comment.ParentCommentID = &pid
	}
	if deletedAt.Valid {
		comment.DeletedAt = &deletedAt.Time
	}
	if deletedByUserID.Valid {
		dbuid, _ := uuid.Parse(deletedByUserID.String)
		comment.DeletedByUserID = &dbuid
	}
	if updatedAt.Valid {
		comment.UpdatedAt = &updatedAt.Time
	}

	comment.User = &user

	if !isAdmin && comment.UserID != userID {
		return nil, errors.New("unauthorized")
	}

	if !isAdmin && comment.DeletedAt != nil {
		sevenDaysAgo := time.Now().AddDate(0, 0, -7)
		if comment.DeletedAt.Before(sevenDaysAgo) {
			return nil, errors.New("comment permanently deleted")
		}
	}

	updateQuery := `
		UPDATE comments
		SET deleted_at = NULL, deleted_by_user_id = NULL
		WHERE id = $1
		RETURNING id, user_id, post_id, parent_comment_id, content, created_at, updated_at, deleted_at, deleted_by_user_id
	`

	var restoredComment models.Comment
	var restoredParentID sql.NullString
	var restoredUpdatedAt sql.NullTime

	err = s.db.QueryRowContext(ctx, updateQuery, commentID).Scan(
		&restoredComment.ID, &restoredComment.UserID, &restoredComment.PostID, &restoredParentID, &restoredComment.Content,
		&restoredComment.CreatedAt, &restoredUpdatedAt, &restoredComment.DeletedAt, &restoredComment.DeletedByUserID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to restore comment: %w", err)
	}

	if restoredParentID.Valid {
		pid, _ := uuid.Parse(restoredParentID.String)
		restoredComment.ParentCommentID = &pid
	}
	if restoredUpdatedAt.Valid {
		restoredComment.UpdatedAt = &restoredUpdatedAt.Time
	}

	restoredComment.User = &user

	links, err := s.getCommentLinks(ctx, commentID)
	if err != nil {
		return nil, err
	}
	restoredComment.Links = links

	// Fetch reactions
	counts, viewerReactions, err := s.getCommentReactions(ctx, commentID, userID)
	if err != nil {
		return nil, err
	}
	restoredComment.ReactionCounts = counts
	restoredComment.ViewerReactions = viewerReactions
	observability.RecordCommentRestored(ctx)

	return &restoredComment, nil
}

// HardDeleteComment permanently deletes a comment and all related data (admin only)
func (s *CommentService) HardDeleteComment(ctx context.Context, commentID uuid.UUID, adminUserID uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Verify comment exists (include soft-deleted comments)
	var exists bool
	err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM comments WHERE id = $1)", commentID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check comment existence: %w", err)
	}
	if !exists {
		return ErrCommentNotFound
	}

	// Create audit log entry BEFORE deleting the comment (FK constraint)
	auditQuery := `
		INSERT INTO audit_logs (admin_user_id, action, related_comment_id, created_at)
		VALUES ($1, 'hard_delete_comment', $2, now())
	`
	_, err = tx.ExecContext(ctx, auditQuery, adminUserID, commentID)
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	// Delete links associated with replies to this comment
	_, err = tx.ExecContext(ctx, "DELETE FROM links WHERE comment_id IN (SELECT id FROM comments WHERE parent_comment_id = $1)", commentID)
	if err != nil {
		return fmt.Errorf("failed to delete reply links: %w", err)
	}

	// Delete reactions on replies to this comment
	_, err = tx.ExecContext(ctx, "DELETE FROM reactions WHERE comment_id IN (SELECT id FROM comments WHERE parent_comment_id = $1)", commentID)
	if err != nil {
		return fmt.Errorf("failed to delete reply reactions: %w", err)
	}

	// Delete mentions from replies to this comment
	_, err = tx.ExecContext(ctx, "DELETE FROM mentions WHERE comment_id IN (SELECT id FROM comments WHERE parent_comment_id = $1)", commentID)
	if err != nil {
		return fmt.Errorf("failed to delete reply mentions: %w", err)
	}

	// Delete notifications related to replies
	_, err = tx.ExecContext(ctx, "DELETE FROM notifications WHERE related_comment_id IN (SELECT id FROM comments WHERE parent_comment_id = $1)", commentID)
	if err != nil {
		return fmt.Errorf("failed to delete reply notifications: %w", err)
	}

	// Delete replies to this comment
	_, err = tx.ExecContext(ctx, "DELETE FROM comments WHERE parent_comment_id = $1", commentID)
	if err != nil {
		return fmt.Errorf("failed to delete replies: %w", err)
	}

	// Delete links associated with this comment
	_, err = tx.ExecContext(ctx, "DELETE FROM links WHERE comment_id = $1", commentID)
	if err != nil {
		return fmt.Errorf("failed to delete comment links: %w", err)
	}

	// Delete reactions on this comment
	_, err = tx.ExecContext(ctx, "DELETE FROM reactions WHERE comment_id = $1", commentID)
	if err != nil {
		return fmt.Errorf("failed to delete comment reactions: %w", err)
	}

	// Delete mentions from this comment
	_, err = tx.ExecContext(ctx, "DELETE FROM mentions WHERE comment_id = $1", commentID)
	if err != nil {
		return fmt.Errorf("failed to delete comment mentions: %w", err)
	}

	// Delete notifications related to this comment
	_, err = tx.ExecContext(ctx, "DELETE FROM notifications WHERE related_comment_id = $1", commentID)
	if err != nil {
		return fmt.Errorf("failed to delete comment notifications: %w", err)
	}

	// Delete the comment
	result, err := tx.ExecContext(ctx, "DELETE FROM comments WHERE id = $1", commentID)
	if err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrCommentNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	observability.RecordCommentDeleted(ctx)

	return nil
}

// AdminRestoreComment restores a soft-deleted comment (admin only) with audit logging
func (s *CommentService) AdminRestoreComment(ctx context.Context, commentID uuid.UUID, adminUserID uuid.UUID) (*models.Comment, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Check if comment exists and is soft-deleted
	var exists bool
	var isDeleted bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM comments WHERE id = $1),
		       EXISTS(SELECT 1 FROM comments WHERE id = $1 AND deleted_at IS NOT NULL)
	`, commentID).Scan(&exists, &isDeleted)
	if err != nil {
		return nil, fmt.Errorf("failed to check comment: %w", err)
	}
	if !exists {
		return nil, ErrCommentNotFound
	}
	if !isDeleted {
		return nil, errors.New("comment is not deleted")
	}

	// Restore the comment
	var comment models.Comment
	var parentID sql.NullString
	var updatedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		UPDATE comments
		SET deleted_at = NULL, deleted_by_user_id = NULL
		WHERE id = $1
		RETURNING id, user_id, post_id, parent_comment_id, content, created_at, updated_at, deleted_at, deleted_by_user_id
	`, commentID).Scan(
		&comment.ID, &comment.UserID, &comment.PostID, &parentID, &comment.Content,
		&comment.CreatedAt, &updatedAt, &comment.DeletedAt, &comment.DeletedByUserID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to restore comment: %w", err)
	}

	if parentID.Valid {
		pid, _ := uuid.Parse(parentID.String)
		comment.ParentCommentID = &pid
	}
	if updatedAt.Valid {
		comment.UpdatedAt = &updatedAt.Time
	}

	// Create audit log entry
	_, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs (admin_user_id, action, related_comment_id, created_at)
		VALUES ($1, 'restore_comment', $2, now())
	`, adminUserID, commentID)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Fetch user info
	var user models.User
	err = s.db.QueryRowContext(ctx, `
		SELECT id, username, COALESCE(email, '') as email, profile_picture_url, bio, is_admin, created_at
		FROM users WHERE id = $1
	`, comment.UserID).Scan(
		&user.ID, &user.Username, &user.Email, &user.ProfilePictureURL, &user.Bio, &user.IsAdmin, &user.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}
	comment.User = &user

	// Fetch links
	links, err := s.getCommentLinks(ctx, commentID)
	if err != nil {
		return nil, err
	}
	comment.Links = links

	// Fetch reactions
	counts, viewerReactions, err := s.getCommentReactions(ctx, commentID, adminUserID)
	if err != nil {
		return nil, err
	}
	comment.ReactionCounts = counts
	comment.ViewerReactions = viewerReactions
	observability.RecordCommentRestored(ctx)

	return &comment, nil
}

// validateUpdateCommentInput validates comment update input
func validateUpdateCommentInput(req *models.UpdateCommentRequest) error {
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

	return nil
}
