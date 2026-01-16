package models

import (
	"time"

	"github.com/google/uuid"
)

// Post represents a post in the system
type Post struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"user_id"`
	SectionID     uuid.UUID `json:"section_id"`
	Content       string    `json:"content"`
	Links         []Link    `json:"links,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
	DeletedByUser *User     `json:"deleted_by_user,omitempty"`
}

// Link represents metadata for a URL
type Link struct {
	ID       uuid.UUID          `json:"id"`
	URL      string             `json:"url"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// CreatePostRequest represents the request body for creating a post
type CreatePostRequest struct {
	SectionID string        `json:"section_id"`
	Content   string        `json:"content"`
	Links     []LinkRequest `json:"links,omitempty"`
}

// LinkRequest represents a link in the request
type LinkRequest struct {
	URL string `json:"url"`
}

// CreatePostResponse represents the response for creating a post
type CreatePostResponse struct {
	Post Post `json:"post"`
}
