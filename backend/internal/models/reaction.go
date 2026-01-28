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

// ReactionUser represents a minimal user payload for reaction tooltips.
type ReactionUser struct {
	ID                uuid.UUID `json:"id"`
	Username          string    `json:"username"`
	ProfilePictureUrl *string   `json:"profile_picture_url,omitempty"`
}

// ReactionGroup represents users grouped by emoji.
type ReactionGroup struct {
	Emoji string         `json:"emoji"`
	Users []ReactionUser `json:"users"`
}

// GetReactionsResponse represents the response for listing reactions on a post or comment.
type GetReactionsResponse struct {
	Reactions []ReactionGroup `json:"reactions"`
}

// CreateReactionRequest represents the request body for creating a reaction
type CreateReactionRequest struct {
	Emoji string `json:"emoji"`
}

// CreateReactionResponse represents the response for creating a reaction
type CreateReactionResponse struct {
	Reaction Reaction `json:"reaction"`
}
