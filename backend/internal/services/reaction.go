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

// RemoveReaction removes a reaction from a post
// Users can only remove their own reactions
func (s *ReactionService) RemoveReaction(ctx context.Context, postID uuid.UUID, emoji string, userID uuid.UUID) error {
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
