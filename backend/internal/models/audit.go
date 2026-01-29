package models

import (
	"time"

	"github.com/google/uuid"
)

// AuditLog represents an audit log entry for admin actions
type AuditLog struct {
	ID               uuid.UUID  `json:"id"`
	AdminUserID      *uuid.UUID `json:"admin_user_id,omitempty"`
	AdminUsername    string     `json:"admin_username,omitempty"`
	Action           string     `json:"action"`
	RelatedPostID    *uuid.UUID `json:"related_post_id,omitempty"`
	RelatedCommentID *uuid.UUID `json:"related_comment_id,omitempty"`
	RelatedUserID    *uuid.UUID `json:"related_user_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// AuditLogsResponse represents the response for listing audit logs
type AuditLogsResponse struct {
	Logs       []*AuditLog `json:"logs"`
	HasMore    bool        `json:"has_more"`
	NextCursor *string     `json:"next_cursor,omitempty"`
}
