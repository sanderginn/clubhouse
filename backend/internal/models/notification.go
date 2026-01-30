package models

import (
	"time"

	"github.com/google/uuid"
)

// Notification represents a notification for a user.
type Notification struct {
	ID               uuid.UUID    `json:"id"`
	UserID           uuid.UUID    `json:"user_id"`
	Type             string       `json:"type"`
	RelatedPostID    *uuid.UUID   `json:"related_post_id,omitempty"`
	RelatedCommentID *uuid.UUID   `json:"related_comment_id,omitempty"`
	RelatedUserID    *uuid.UUID   `json:"related_user_id,omitempty"`
	RelatedUser      *UserSummary `json:"related_user,omitempty"`
	ContentExcerpt   *string      `json:"content_excerpt,omitempty"`
	ReadAt           *time.Time   `json:"read_at,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
}

// NotificationMeta represents pagination metadata for notifications.
type NotificationMeta struct {
	Cursor      *string `json:"cursor,omitempty"`
	HasMore     bool    `json:"has_more"`
	UnreadCount int     `json:"unread_count"`
}

// GetNotificationsResponse represents the response for notifications.
type GetNotificationsResponse struct {
	Notifications []Notification   `json:"notifications"`
	Meta          NotificationMeta `json:"meta"`
}

// UpdateNotificationResponse represents the response for updating a notification.
type UpdateNotificationResponse struct {
	Notification Notification `json:"notification"`
}
