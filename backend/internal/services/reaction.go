package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
)

// ReactionService handles reaction-related operations
type ReactionService struct {
	db *sql.DB
}

// NewReactionService creates a new reaction service
func NewReactionService(db *sql.DB) *ReactionService {
	return &ReactionService{db: db}
}

// AddReactionToPost adds a reaction to a post
func (s *ReactionService) AddReactionToPost(ctx context.Context, postID uuid.UUID, userID uuid.UUID, emoji string) (*models.Reaction, error) {
	if err := validateEmoji(emoji); err != nil {
		return nil, err
	}

	if err := s.verifyPostExists(ctx, postID); err != nil {
		return nil, err
	}

	existingReaction, err := s.getExistingPostReaction(ctx, postID, userID, emoji)
	if err != nil {
		return nil, err
	}

	if existingReaction != nil {
		if existingReaction.DeletedAt != nil {
			return s.restoreReaction(ctx, existingReaction.ID)
		}
		return existingReaction, nil
	}

	return s.createPostReaction(ctx, postID, userID, emoji)
}

// GetPostReactions retrieves reactions for a post grouped by emoji.
func (s *ReactionService) GetPostReactions(ctx context.Context, postID uuid.UUID) ([]models.ReactionGroup, error) {
	if err := s.verifyPostExists(ctx, postID); err != nil {
		return nil, err
	}
	return s.getReactions(ctx, "post_id", postID)
}

// RemoveReactionFromPost removes a reaction from a post
// Users can only remove their own reactions
func (s *ReactionService) RemoveReactionFromPost(ctx context.Context, postID uuid.UUID, emoji string, userID uuid.UUID) error {
	query := `
		DELETE FROM reactions
		WHERE post_id = $1 AND emoji = $2 AND user_id = $3 AND deleted_at IS NULL
	`

	result, err := s.db.ExecContext(ctx, query, postID, emoji, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("reaction not found")
	}

	return nil
}

// AddReactionToComment adds a reaction to a comment
func (s *ReactionService) AddReactionToComment(ctx context.Context, commentID uuid.UUID, userID uuid.UUID, emoji string) (*models.Reaction, error) {
	if err := validateEmoji(emoji); err != nil {
		return nil, err
	}

	if err := s.verifyCommentExists(ctx, commentID); err != nil {
		return nil, err
	}

	existingReaction, err := s.getExistingCommentReaction(ctx, commentID, userID, emoji)
	if err != nil {
		return nil, err
	}

	if existingReaction != nil {
		if existingReaction.DeletedAt != nil {
			return s.restoreReaction(ctx, existingReaction.ID)
		}
		return existingReaction, nil
	}

	return s.createCommentReaction(ctx, commentID, userID, emoji)
}

// GetCommentReactions retrieves reactions for a comment grouped by emoji.
func (s *ReactionService) GetCommentReactions(ctx context.Context, commentID uuid.UUID) ([]models.ReactionGroup, error) {
	if err := s.verifyCommentExists(ctx, commentID); err != nil {
		return nil, err
	}
	return s.getReactions(ctx, "comment_id", commentID)
}

// RemoveReactionFromComment removes a reaction from a comment
// Users can only remove their own reactions
func (s *ReactionService) RemoveReactionFromComment(ctx context.Context, commentID uuid.UUID, emoji string, userID uuid.UUID) error {
	query := `
		DELETE FROM reactions
		WHERE comment_id = $1 AND emoji = $2 AND user_id = $3 AND deleted_at IS NULL
	`

	result, err := s.db.ExecContext(ctx, query, commentID, emoji, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("reaction not found")
	}

	return nil
}

func validateEmoji(emoji string) error {
	emoji = strings.TrimSpace(emoji)
	if emoji == "" {
		return fmt.Errorf("emoji is required")
	}
	if len(emoji) > 10 {
		return fmt.Errorf("emoji must be 10 characters or less")
	}
	return nil
}

func (s *ReactionService) verifyPostExists(ctx context.Context, postID uuid.UUID) error {
	var postExists bool
	err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1 AND deleted_at IS NULL)",
		postID,
	).Scan(&postExists)
	if err != nil {
		return fmt.Errorf("failed to check post existence: %w", err)
	}
	if !postExists {
		return errors.New("post not found")
	}
	return nil
}

func (s *ReactionService) getExistingPostReaction(ctx context.Context, postID uuid.UUID, userID uuid.UUID, emoji string) (*models.Reaction, error) {
	query := `
		SELECT id, user_id, post_id, comment_id, emoji, created_at, deleted_at
		FROM reactions
		WHERE user_id = $1 AND post_id = $2 AND emoji = $3
	`

	var reaction models.Reaction
	err := s.db.QueryRowContext(ctx, query, userID, postID, emoji).Scan(
		&reaction.ID, &reaction.UserID, &reaction.PostID, &reaction.CommentID,
		&reaction.Emoji, &reaction.CreatedAt, &reaction.DeletedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to check existing reaction: %w", err)
	}

	return &reaction, nil
}

func (s *ReactionService) restoreReaction(ctx context.Context, reactionID uuid.UUID) (*models.Reaction, error) {
	query := `
		UPDATE reactions
		SET deleted_at = NULL
		WHERE id = $1
		RETURNING id, user_id, post_id, comment_id, emoji, created_at, deleted_at
	`

	var reaction models.Reaction
	err := s.db.QueryRowContext(ctx, query, reactionID).Scan(
		&reaction.ID, &reaction.UserID, &reaction.PostID, &reaction.CommentID,
		&reaction.Emoji, &reaction.CreatedAt, &reaction.DeletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to restore reaction: %w", err)
	}

	return &reaction, nil
}

func (s *ReactionService) createPostReaction(ctx context.Context, postID uuid.UUID, userID uuid.UUID, emoji string) (*models.Reaction, error) {
	query := `
		INSERT INTO reactions (id, user_id, post_id, emoji, created_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING id, user_id, post_id, comment_id, emoji, created_at, deleted_at
	`

	reactionID := uuid.New()
	var reaction models.Reaction
	err := s.db.QueryRowContext(ctx, query, reactionID, userID, postID, emoji).Scan(
		&reaction.ID, &reaction.UserID, &reaction.PostID, &reaction.CommentID,
		&reaction.Emoji, &reaction.CreatedAt, &reaction.DeletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create reaction: %w", err)
	}

	return &reaction, nil
}

func (s *ReactionService) verifyCommentExists(ctx context.Context, commentID uuid.UUID) error {
	var commentExists bool
	err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM comments WHERE id = $1 AND deleted_at IS NULL)",
		commentID,
	).Scan(&commentExists)
	if err != nil {
		return fmt.Errorf("failed to check comment existence: %w", err)
	}
	if !commentExists {
		return errors.New("comment not found")
	}
	return nil
}

func (s *ReactionService) getExistingCommentReaction(ctx context.Context, commentID uuid.UUID, userID uuid.UUID, emoji string) (*models.Reaction, error) {
	query := `
		SELECT id, user_id, post_id, comment_id, emoji, created_at, deleted_at
		FROM reactions
		WHERE user_id = $1 AND comment_id = $2 AND emoji = $3
	`

	var reaction models.Reaction
	err := s.db.QueryRowContext(ctx, query, userID, commentID, emoji).Scan(
		&reaction.ID, &reaction.UserID, &reaction.PostID, &reaction.CommentID,
		&reaction.Emoji, &reaction.CreatedAt, &reaction.DeletedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to check existing reaction: %w", err)
	}

	return &reaction, nil
}

func (s *ReactionService) createCommentReaction(ctx context.Context, commentID uuid.UUID, userID uuid.UUID, emoji string) (*models.Reaction, error) {
	query := `
		INSERT INTO reactions (id, user_id, comment_id, emoji, created_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING id, user_id, post_id, comment_id, emoji, created_at, deleted_at
	`

	reactionID := uuid.New()
	var reaction models.Reaction
	err := s.db.QueryRowContext(ctx, query, reactionID, userID, commentID, emoji).Scan(
		&reaction.ID, &reaction.UserID, &reaction.PostID, &reaction.CommentID,
		&reaction.Emoji, &reaction.CreatedAt, &reaction.DeletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create reaction: %w", err)
	}

	return &reaction, nil
}

func (s *ReactionService) getReactions(ctx context.Context, column string, id uuid.UUID) ([]models.ReactionGroup, error) {
	query := fmt.Sprintf(`
		SELECT r.emoji, u.id, u.username, u.profile_picture_url
		FROM reactions r
		JOIN users u ON r.user_id = u.id
		WHERE r.%s = $1 AND r.deleted_at IS NULL
		ORDER BY r.emoji ASC, r.created_at ASC
	`, column)

	rows, err := s.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query reactions: %w", err)
	}
	defer rows.Close()

	groups := []models.ReactionGroup{}
	groupIndex := map[string]int{}

	for rows.Next() {
		var emoji string
		var user models.ReactionUser
		if err := rows.Scan(&emoji, &user.ID, &user.Username, &user.ProfilePictureUrl); err != nil {
			return nil, fmt.Errorf("failed to scan reaction user: %w", err)
		}

		if idx, ok := groupIndex[emoji]; ok {
			groups[idx].Users = append(groups[idx].Users, user)
			continue
		}

		groupIndex[emoji] = len(groups)
		groups = append(groups, models.ReactionGroup{
			Emoji: emoji,
			Users: []models.ReactionUser{user},
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate reactions: %w", err)
	}

	return groups, nil
}
