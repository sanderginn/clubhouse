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
	// Validate input
	if err := validateCreateCommentInput(req); err != nil {
		return nil, err
	}

	// Parse and validate post ID
	postID, err := uuid.Parse(req.PostID)
	if err != nil {
		return nil, fmt.Errorf("invalid post id")
	}

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

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

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

		for _, linkReq := range req.Links {
			linkID := uuid.New()

			// Insert link for comment
			linkQuery := `
				INSERT INTO links (id, comment_id, url, created_at)
				VALUES ($1, $2, $3, now())
				RETURNING id, url, created_at
			`

			var link models.Link
			err := tx.QueryRowContext(ctx, linkQuery, linkID, commentID, linkReq.URL).
				Scan(&link.ID, &link.URL, &link.CreatedAt)

			if err != nil {
				return nil, fmt.Errorf("failed to create link: %w", err)
			}

			comment.Links = append(comment.Links, link)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &comment, nil
}

// GetCommentByID retrieves a comment by ID with all related data
func (s *CommentService) GetCommentByID(ctx context.Context, commentID uuid.UUID) (*models.Comment, error) {
	query := `
		SELECT 
			c.id, c.user_id, c.post_id, c.parent_comment_id, c.content,
			c.created_at, c.updated_at, c.deleted_at, c.deleted_by_user_id,
			u.id, u.username, u.email, u.profile_picture_url, u.bio, u.is_admin, u.created_at
		FROM comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.id = $1 AND c.deleted_at IS NULL
	`

	var comment models.Comment
	var user models.User

	err := s.db.QueryRowContext(ctx, query, commentID).Scan(
		&comment.ID, &comment.UserID, &comment.PostID, &comment.ParentCommentID, &comment.Content,
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

	// Fetch links for this comment
	links, err := s.getCommentLinks(ctx, commentID)
	if err != nil {
		return nil, err
	}
	comment.Links = links

	return &comment, nil
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
func (s *CommentService) GetThreadComments(ctx context.Context, postID uuid.UUID, limit int, cursor *string) ([]models.Comment, *string, bool, error) {
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
			u.id, u.username, u.email, u.profile_picture_url, u.bio, u.is_admin, u.created_at
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
		replies, err := s.getCommentReplies(ctx, comments[i].ID)
		if err != nil {
			return nil, nil, false, fmt.Errorf("failed to get comment replies: %w", err)
		}
		comments[i].Replies = replies
	}

	return comments, nextCursor, hasMore, nil
}

// getCommentReplies retrieves all replies to a comment
func (s *CommentService) getCommentReplies(ctx context.Context, parentCommentID uuid.UUID) ([]models.Comment, error) {
	query := `
		SELECT 
			c.id, c.user_id, c.post_id, c.parent_comment_id, c.content, 
			c.created_at, c.updated_at, c.deleted_at, c.deleted_by_user_id,
			u.id, u.username, u.email, u.profile_picture_url, u.bio, u.is_admin, u.created_at
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

		replies = append(replies, c)
	}

	return replies, rows.Err()
}
