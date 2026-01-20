package services

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

const (
	notificationTypeNewPost    = "new_post"
	notificationTypeNewComment = "new_comment"
	notificationTypeMention    = "mention"
	notificationTypeReaction   = "reaction"
)

// NotificationService handles notification creation.
type NotificationService struct {
	db *sql.DB
}

// NewNotificationService creates a new notification service.
func NewNotificationService(db *sql.DB) *NotificationService {
	return &NotificationService{db: db}
}

// CreateNotificationsForNewPost creates notifications for all subscribed users in a section.
func (s *NotificationService) CreateNotificationsForNewPost(ctx context.Context, postID, sectionID, authorID uuid.UUID) error {
	query := `
		INSERT INTO notifications (user_id, type, related_post_id, related_user_id)
		SELECT u.id, $1, $2, $3
		FROM users u
		WHERE u.deleted_at IS NULL
		  AND u.approved_at IS NOT NULL
		  AND u.id <> $3
		  AND NOT EXISTS (
				SELECT 1 FROM section_subscriptions ss
				WHERE ss.user_id = u.id AND ss.section_id = $4
		  )
	`

	_, err := s.db.ExecContext(ctx, query, notificationTypeNewPost, postID, authorID, sectionID)
	if err != nil {
		return fmt.Errorf("failed to create new post notifications: %w", err)
	}

	return nil
}

// CreateNotificationForPostComment notifies a post owner about a new comment.
func (s *NotificationService) CreateNotificationForPostComment(ctx context.Context, postID, commentID, commenterID uuid.UUID) error {
	postOwnerID, sectionID, err := s.getPostOwnerAndSectionID(ctx, postID)
	if err != nil {
		return err
	}
	if postOwnerID == commenterID {
		return nil
	}

	subscribed, err := s.isUserSubscribedToSection(ctx, postOwnerID, sectionID)
	if err != nil {
		return err
	}
	if !subscribed {
		return nil
	}

	return s.insertNotification(ctx, postOwnerID, notificationTypeNewComment, &postID, &commentID, &commenterID)
}

// CreateMentionNotifications creates notifications for mentioned users.
func (s *NotificationService) CreateMentionNotifications(ctx context.Context, mentionedUserIDs []uuid.UUID, mentionerID uuid.UUID, sectionID uuid.UUID, postID uuid.UUID, commentID *uuid.UUID) error {
	if len(mentionedUserIDs) == 0 {
		return nil
	}

	for _, mentionedUserID := range mentionedUserIDs {
		if mentionedUserID == mentionerID {
			continue
		}

		subscribed, err := s.isUserSubscribedToSection(ctx, mentionedUserID, sectionID)
		if err != nil {
			return err
		}
		if !subscribed {
			continue
		}

		postIDCopy := postID
		if err := s.insertNotification(ctx, mentionedUserID, notificationTypeMention, &postIDCopy, commentID, &mentionerID); err != nil {
			return err
		}
	}

	return nil
}

// CreateNotificationForPostReaction notifies a post owner about a reaction.
func (s *NotificationService) CreateNotificationForPostReaction(ctx context.Context, postID, reactorID uuid.UUID) error {
	postOwnerID, sectionID, err := s.getPostOwnerAndSectionID(ctx, postID)
	if err != nil {
		return err
	}
	if postOwnerID == reactorID {
		return nil
	}

	subscribed, err := s.isUserSubscribedToSection(ctx, postOwnerID, sectionID)
	if err != nil {
		return err
	}
	if !subscribed {
		return nil
	}

	return s.insertNotification(ctx, postOwnerID, notificationTypeReaction, &postID, nil, &reactorID)
}

// CreateNotificationForCommentReaction notifies a comment owner about a reaction.
func (s *NotificationService) CreateNotificationForCommentReaction(ctx context.Context, commentID, reactorID uuid.UUID) error {
	commentOwnerID, postID, sectionID, err := s.getCommentOwnerPostAndSection(ctx, commentID)
	if err != nil {
		return err
	}
	if commentOwnerID == reactorID {
		return nil
	}

	subscribed, err := s.isUserSubscribedToSection(ctx, commentOwnerID, sectionID)
	if err != nil {
		return err
	}
	if !subscribed {
		return nil
	}

	return s.insertNotification(ctx, commentOwnerID, notificationTypeReaction, &postID, &commentID, &reactorID)
}

func (s *NotificationService) insertNotification(ctx context.Context, userID uuid.UUID, notificationType string, postID *uuid.UUID, commentID *uuid.UUID, relatedUserID *uuid.UUID) error {
	query := `
		INSERT INTO notifications (user_id, type, related_post_id, related_comment_id, related_user_id)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := s.db.ExecContext(ctx, query, userID, notificationType, postID, commentID, relatedUserID)
	if err != nil {
		return fmt.Errorf("failed to insert notification: %w", err)
	}

	return nil
}

func (s *NotificationService) getPostOwnerAndSectionID(ctx context.Context, postID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	query := `
		SELECT user_id, section_id
		FROM posts
		WHERE id = $1 AND deleted_at IS NULL
	`

	var ownerID uuid.UUID
	var sectionID uuid.UUID
	if err := s.db.QueryRowContext(ctx, query, postID).Scan(&ownerID, &sectionID); err != nil {
		return uuid.UUID{}, uuid.UUID{}, fmt.Errorf("failed to get post owner: %w", err)
	}

	return ownerID, sectionID, nil
}

func (s *NotificationService) getCommentOwnerPostAndSection(ctx context.Context, commentID uuid.UUID) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	query := `
		SELECT c.user_id, c.post_id, p.section_id
		FROM comments c
		JOIN posts p ON p.id = c.post_id
		WHERE c.id = $1 AND c.deleted_at IS NULL AND p.deleted_at IS NULL
	`

	var ownerID uuid.UUID
	var postID uuid.UUID
	var sectionID uuid.UUID
	if err := s.db.QueryRowContext(ctx, query, commentID).Scan(&ownerID, &postID, &sectionID); err != nil {
		return uuid.UUID{}, uuid.UUID{}, uuid.UUID{}, fmt.Errorf("failed to get comment owner: %w", err)
	}

	return ownerID, postID, sectionID, nil
}

func (s *NotificationService) isUserSubscribedToSection(ctx context.Context, userID uuid.UUID, sectionID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM users u
			WHERE u.id = $1
			  AND u.deleted_at IS NULL
			  AND u.approved_at IS NOT NULL
			  AND NOT EXISTS (
					SELECT 1
					FROM section_subscriptions ss
					WHERE ss.user_id = u.id AND ss.section_id = $2
			  )
		)
	`

	var subscribed bool
	if err := s.db.QueryRowContext(ctx, query, userID, sectionID).Scan(&subscribed); err != nil {
		return false, fmt.Errorf("failed to check subscription: %w", err)
	}

	return subscribed, nil
}
