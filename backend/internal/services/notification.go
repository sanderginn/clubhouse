package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const (
	notificationTypeNewPost    = "new_post"
	notificationTypeNewComment = "new_comment"
	notificationTypeMention    = "mention"
	notificationTypeReaction   = "reaction"
)

// NotificationService handles notification creation.
type NotificationService struct {
	db   *sql.DB
	push *PushService
}

// NewNotificationService creates a new notification service.
func NewNotificationService(db *sql.DB, pushService *PushService) *NotificationService {
	return &NotificationService{db: db, push: pushService}
}

// CreateNotificationsForNewPost creates notifications for all subscribed users in a section.
func (s *NotificationService) CreateNotificationsForNewPost(ctx context.Context, postID, sectionID, authorID uuid.UUID) error {
	ctx, span := otel.Tracer("clubhouse.notifications").Start(ctx, "NotificationService.CreateNotificationsForNewPost")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.String("section_id", sectionID.String()),
		attribute.String("author_id", authorID.String()),
	)
	defer span.End()

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

	result, err := s.db.ExecContext(ctx, query, notificationTypeNewPost, postID, authorID, sectionID)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to create new post notifications: %w", err)
	}
	if rows, err := result.RowsAffected(); err == nil {
		observability.RecordNotificationsCreated(ctx, notificationTypeNewPost, rows)
	}

	s.sendPushToSection(ctx, notificationTypeNewPost, postID, nil, &authorID, sectionID, authorID)

	return nil
}

// CreateNotificationForPostComment notifies a post owner about a new comment.
func (s *NotificationService) CreateNotificationForPostComment(ctx context.Context, postID, commentID, commenterID uuid.UUID) error {
	ctx, span := otel.Tracer("clubhouse.notifications").Start(ctx, "NotificationService.CreateNotificationForPostComment")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.String("comment_id", commentID.String()),
		attribute.String("commenter_id", commenterID.String()),
	)
	defer span.End()

	postOwnerID, sectionID, err := s.getPostOwnerAndSectionID(ctx, postID)
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	if postOwnerID == commenterID {
		return nil
	}

	subscribed, err := s.isUserSubscribedToSection(ctx, postOwnerID, sectionID)
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	if !subscribed {
		return nil
	}

	if err := s.insertNotification(ctx, postOwnerID, notificationTypeNewComment, &postID, &commentID, &commenterID); err != nil {
		recordSpanError(span, err)
		return err
	}
	return nil
}

// CreateMentionNotifications creates notifications for mentioned users.
func (s *NotificationService) CreateMentionNotifications(ctx context.Context, mentionedUserIDs []uuid.UUID, mentionerID uuid.UUID, sectionID uuid.UUID, postID uuid.UUID, commentID *uuid.UUID) error {
	ctx, span := otel.Tracer("clubhouse.notifications").Start(ctx, "NotificationService.CreateMentionNotifications")
	span.SetAttributes(
		attribute.Int("mentioned_user_count", len(mentionedUserIDs)),
		attribute.String("mentioner_id", mentionerID.String()),
		attribute.String("section_id", sectionID.String()),
		attribute.String("post_id", postID.String()),
		attribute.Bool("has_comment_id", commentID != nil),
	)
	defer span.End()

	if len(mentionedUserIDs) == 0 {
		return nil
	}

	for _, mentionedUserID := range mentionedUserIDs {
		if mentionedUserID == mentionerID {
			continue
		}

		subscribed, err := s.isUserSubscribedToSection(ctx, mentionedUserID, sectionID)
		if err != nil {
			recordSpanError(span, err)
			return err
		}
		if !subscribed {
			continue
		}

		postIDCopy := postID
		if err := s.insertNotification(ctx, mentionedUserID, notificationTypeMention, &postIDCopy, commentID, &mentionerID); err != nil {
			recordSpanError(span, err)
			return err
		}
	}

	return nil
}

// CreateNotificationForPostReaction notifies a post owner about a reaction.
func (s *NotificationService) CreateNotificationForPostReaction(ctx context.Context, postID, reactorID uuid.UUID) error {
	ctx, span := otel.Tracer("clubhouse.notifications").Start(ctx, "NotificationService.CreateNotificationForPostReaction")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.String("reactor_id", reactorID.String()),
	)
	defer span.End()

	postOwnerID, sectionID, err := s.getPostOwnerAndSectionID(ctx, postID)
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	if postOwnerID == reactorID {
		return nil
	}

	subscribed, err := s.isUserSubscribedToSection(ctx, postOwnerID, sectionID)
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	if !subscribed {
		return nil
	}

	if err := s.insertNotification(ctx, postOwnerID, notificationTypeReaction, &postID, nil, &reactorID); err != nil {
		recordSpanError(span, err)
		return err
	}
	return nil
}

// CreateNotificationForCommentReaction notifies a comment owner about a reaction.
func (s *NotificationService) CreateNotificationForCommentReaction(ctx context.Context, commentID, reactorID uuid.UUID) error {
	ctx, span := otel.Tracer("clubhouse.notifications").Start(ctx, "NotificationService.CreateNotificationForCommentReaction")
	span.SetAttributes(
		attribute.String("comment_id", commentID.String()),
		attribute.String("reactor_id", reactorID.String()),
	)
	defer span.End()

	commentOwnerID, postID, sectionID, err := s.getCommentOwnerPostAndSection(ctx, commentID)
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.String("section_id", sectionID.String()),
	)
	if commentOwnerID == reactorID {
		return nil
	}

	subscribed, err := s.isUserSubscribedToSection(ctx, commentOwnerID, sectionID)
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	if !subscribed {
		return nil
	}

	if err := s.insertNotification(ctx, commentOwnerID, notificationTypeReaction, &postID, &commentID, &reactorID); err != nil {
		recordSpanError(span, err)
		return err
	}
	return nil
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

	observability.RecordNotificationsCreated(ctx, notificationType, 1)
	s.sendPush(ctx, userID, notificationType, postID, commentID, relatedUserID)

	return nil
}

func (s *NotificationService) sendPush(ctx context.Context, userID uuid.UUID, notificationType string, postID *uuid.UUID, commentID *uuid.UUID, relatedUserID *uuid.UUID) {
	if s.push == nil {
		return
	}

	payload := buildPushPayload(notificationType, postID, commentID, relatedUserID)
	result, err := s.push.SendNotification(ctx, userID, payload)
	if result.Delivered > 0 {
		observability.RecordNotificationDelivered(ctx, "push", result.Delivered)
	}
	for failureType, count := range result.FailedByType {
		observability.RecordNotificationDeliveryFailed(ctx, "push", failureType, count)
	}
	if err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message:    "failed to send push notification",
			Code:       "PUSH_SEND_FAILED",
			StatusCode: http.StatusInternalServerError,
			UserID:     userID.String(),
			Err:        err,
		})
		return
	}
}

func (s *NotificationService) sendPushToSection(ctx context.Context, notificationType string, postID uuid.UUID, commentID *uuid.UUID, relatedUserID *uuid.UUID, sectionID uuid.UUID, authorID uuid.UUID) {
	if s.push == nil {
		return
	}

	userIDs, err := s.getSubscribedUserIDs(ctx, sectionID, authorID)
	if err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message:    "failed to load push subscription users",
			Code:       "PUSH_SUBSCRIPTION_FETCH_FAILED",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		})
		return
	}
	if len(userIDs) == 0 {
		return
	}

	payload := buildPushPayload(notificationType, &postID, commentID, relatedUserID)
	result, err := s.push.SendNotificationToUsers(ctx, userIDs, payload)
	if result.Delivered > 0 {
		observability.RecordNotificationDelivered(ctx, "push", result.Delivered)
	}
	for failureType, count := range result.FailedByType {
		observability.RecordNotificationDeliveryFailed(ctx, "push", failureType, count)
	}
	if err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message:    "failed to send push notifications",
			Code:       "PUSH_SEND_FAILED",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		})
		return
	}
}

func (s *NotificationService) getSubscribedUserIDs(ctx context.Context, sectionID uuid.UUID, authorID uuid.UUID) ([]uuid.UUID, error) {
	query := `
		SELECT u.id
		FROM users u
		WHERE u.deleted_at IS NULL
		  AND u.approved_at IS NOT NULL
		  AND u.id <> $1
		  AND NOT EXISTS (
				SELECT 1 FROM section_subscriptions ss
				WHERE ss.user_id = u.id AND ss.section_id = $2
		  )
	`

	rows, err := s.db.QueryContext(ctx, query, authorID, sectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	userIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return userIDs, nil
}

func buildPushPayload(notificationType string, postID *uuid.UUID, commentID *uuid.UUID, relatedUserID *uuid.UUID) models.PushNotificationPayload {
	payload := models.PushNotificationPayload{
		Type:          notificationType,
		PostID:        postID,
		CommentID:     commentID,
		RelatedUserID: relatedUserID,
	}

	switch notificationType {
	case notificationTypeNewPost:
		payload.Title = "New post"
		payload.Body = "There is a new post in your community."
	case notificationTypeNewComment:
		payload.Title = "New comment"
		payload.Body = "Someone commented on your post."
	case notificationTypeMention:
		payload.Title = "New mention"
		payload.Body = "You were mentioned in a post or comment."
	case notificationTypeReaction:
		payload.Title = "New reaction"
		payload.Body = "Someone reacted to your post or comment."
	}

	return payload
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

// GetNotifications retrieves notifications for a user with cursor-based pagination and unread count.
func (s *NotificationService) GetNotifications(ctx context.Context, userID uuid.UUID, limit int, cursor *string) ([]models.Notification, *string, bool, int, error) {
	ctx, span := otel.Tracer("clubhouse.notifications").Start(ctx, "NotificationService.GetNotifications")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.Int("limit", limit),
		attribute.Bool("has_cursor", cursor != nil && *cursor != ""),
	)
	defer span.End()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	unreadCount, err := s.getUnreadCount(ctx, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, nil, false, 0, err
	}

	query := `
		SELECT id, user_id, type, related_post_id, related_comment_id, related_user_id, read_at, created_at
		FROM notifications
		WHERE user_id = $1
	`

	args := []interface{}{userID}
	argIndex := 2

	if cursor != nil && *cursor != "" {
		cursorTime, cursorID, err := s.resolveNotificationCursor(ctx, userID, *cursor)
		if err != nil {
			recordSpanError(span, err)
			return nil, nil, false, unreadCount, err
		}

		query += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", argIndex, argIndex+1)
		args = append(args, cursorTime, cursorID)
		argIndex += 2
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", argIndex)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		recordSpanError(span, err)
		return nil, nil, false, unreadCount, fmt.Errorf("failed to query notifications: %w", err)
	}
	defer rows.Close()

	notifications := make([]models.Notification, 0)
	for rows.Next() {
		var notification models.Notification
		var relatedPostID sql.NullString
		var relatedCommentID sql.NullString
		var relatedUserID sql.NullString
		var readAt sql.NullTime

		if err := rows.Scan(
			&notification.ID,
			&notification.UserID,
			&notification.Type,
			&relatedPostID,
			&relatedCommentID,
			&relatedUserID,
			&readAt,
			&notification.CreatedAt,
		); err != nil {
			recordSpanError(span, err)
			return nil, nil, false, unreadCount, fmt.Errorf("failed to scan notification: %w", err)
		}

		if relatedPostID.Valid {
			id, _ := uuid.Parse(relatedPostID.String)
			notification.RelatedPostID = &id
		}
		if relatedCommentID.Valid {
			id, _ := uuid.Parse(relatedCommentID.String)
			notification.RelatedCommentID = &id
		}
		if relatedUserID.Valid {
			id, _ := uuid.Parse(relatedUserID.String)
			notification.RelatedUserID = &id
		}
		if readAt.Valid {
			notification.ReadAt = &readAt.Time
		}

		notifications = append(notifications, notification)
	}

	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, nil, false, unreadCount, fmt.Errorf("error iterating notifications: %w", err)
	}

	hasMore := len(notifications) > limit
	if hasMore {
		notifications = notifications[:limit]
	}

	var nextCursor *string
	if hasMore && len(notifications) > 0 {
		last := notifications[len(notifications)-1]
		cursorValue := fmt.Sprintf("%s|%s", last.CreatedAt.UTC().Format(time.RFC3339Nano), last.ID.String())
		nextCursor = &cursorValue
	}

	return notifications, nextCursor, hasMore, unreadCount, nil
}

func (s *NotificationService) getUnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL",
		userID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}
	return count, nil
}

func (s *NotificationService) resolveNotificationCursor(ctx context.Context, userID uuid.UUID, cursor string) (time.Time, uuid.UUID, error) {
	if strings.Contains(cursor, "|") {
		parts := strings.SplitN(cursor, "|", 2)
		if len(parts) != 2 {
			return time.Time{}, uuid.UUID{}, errors.New("invalid cursor")
		}
		parsedTime, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return time.Time{}, uuid.UUID{}, errors.New("invalid cursor")
		}
		parsedID, err := uuid.Parse(parts[1])
		if err != nil {
			return time.Time{}, uuid.UUID{}, errors.New("invalid cursor")
		}
		return parsedTime, parsedID, nil
	}

	cursorID, err := uuid.Parse(cursor)
	if err != nil {
		return time.Time{}, uuid.UUID{}, errors.New("invalid cursor")
	}

	var cursorTime sql.NullTime
	err = s.db.QueryRowContext(ctx,
		"SELECT created_at FROM notifications WHERE id = $1 AND user_id = $2",
		cursorID, userID,
	).Scan(&cursorTime)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, uuid.UUID{}, errors.New("cursor not found")
	}
	if err != nil {
		return time.Time{}, uuid.UUID{}, fmt.Errorf("failed to get cursor time: %w", err)
	}

	return cursorTime.Time, cursorID, nil
}

// MarkNotificationRead sets read_at for a notification and returns the updated notification.
func (s *NotificationService) MarkNotificationRead(ctx context.Context, userID uuid.UUID, notificationID uuid.UUID) (*models.Notification, error) {
	ctx, span := otel.Tracer("clubhouse.notifications").Start(ctx, "NotificationService.MarkNotificationRead")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("notification_id", notificationID.String()),
	)
	defer span.End()

	var ownerID uuid.UUID
	if err := s.db.QueryRowContext(ctx, "SELECT user_id FROM notifications WHERE id = $1", notificationID).Scan(&ownerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := errors.New("notification not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to load notification owner: %w", err)
	}

	if ownerID != userID {
		forbiddenErr := errors.New("forbidden")
		recordSpanError(span, forbiddenErr)
		return nil, forbiddenErr
	}

	query := `
		UPDATE notifications
		SET read_at = CASE WHEN read_at IS NULL THEN now() ELSE read_at END
		WHERE id = $1 AND user_id = $2
		RETURNING id, user_id, type, related_post_id, related_comment_id, related_user_id, read_at, created_at
	`

	var notification models.Notification
	var relatedPostID sql.NullString
	var relatedCommentID sql.NullString
	var relatedUserID sql.NullString
	var readAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, notificationID, userID).Scan(
		&notification.ID,
		&notification.UserID,
		&notification.Type,
		&relatedPostID,
		&relatedCommentID,
		&relatedUserID,
		&readAt,
		&notification.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := errors.New("notification not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to mark notification read: %w", err)
	}

	if relatedPostID.Valid {
		id, _ := uuid.Parse(relatedPostID.String)
		notification.RelatedPostID = &id
	}
	if relatedCommentID.Valid {
		id, _ := uuid.Parse(relatedCommentID.String)
		notification.RelatedCommentID = &id
	}
	if relatedUserID.Valid {
		id, _ := uuid.Parse(relatedUserID.String)
		notification.RelatedUserID = &id
	}
	if readAt.Valid {
		notification.ReadAt = &readAt.Time
	}

	return &notification, nil
}
