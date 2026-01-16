package models

import (
	"time"

	"github.com/google/uuid"
)

// Comment represents a comment in the system
type Comment struct {
	ID                uuid.UUID  `json:"id"`
	UserID            uuid.UUID  `json:"user_id"`
	PostID            uuid.UUID  `json:"post_id"`
	ParentCommentID   *uuid.UUID `json:"parent_comment_id,omitempty"`
	Content           string     `json:"content"`
	Links             []Link     `json:"links,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         *time.Time `json:"updated_at,omitempty"`
	DeletedAt         *time.Time `json:"deleted_at,omitempty"`
	DeletedByUserID   *uuid.UUID `json:"deleted_by_user_id,omitempty"`
	User              *User      `json:"user,omitempty"`
}

// CreateCommentRequest represents the request body for creating a comment
type CreateCommentRequest struct {
	PostID          string        `json:"post_id"`
	ParentCommentID *string       `json:"parent_comment_id,omitempty"`
	Content         string        `json:"content"`
	Links           []LinkRequest `json:"links,omitempty"`
}

// CreateCommentResponse represents the response for creating a comment
type CreateCommentResponse struct {
	Comment Comment `json:"comment"`
}

// GetCommentResponse represents the response for getting a single comment
type GetCommentResponse struct {
	Comment *Comment `json:"comment"`
}
