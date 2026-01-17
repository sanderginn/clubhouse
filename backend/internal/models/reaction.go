package models

import (
	"time"

	"github.com/google/uuid"
)

// Reaction represents a reaction on a post or comment
type Reaction struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	PostID    *uuid.UUID `json:"post_id,omitempty"`
	CommentID *uuid.UUID `json:"comment_id,omitempty"`
	Emoji     string     `json:"emoji"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// CreateReactionRequest represents the request body for creating a reaction
type CreateReactionRequest struct {
	Emoji string `json:"emoji"`
}

// CreateReactionResponse represents the response for creating a reaction
type CreateReactionResponse struct {
	Reaction Reaction `json:"reaction"`
}
