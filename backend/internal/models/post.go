package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Post represents a post in the system
type Post struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"user_id"`
	SectionID       uuid.UUID  `json:"section_id"`
	Content         string     `json:"content"`
	Links           []Link     `json:"links,omitempty"`
	CommentCount    int        `json:"comment_count"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
	DeletedByUserID *uuid.UUID `json:"deleted_by_user_id,omitempty"`
	User            *User      `json:"user,omitempty"`
}

// Link represents metadata for a URL
type Link struct {
	ID        uuid.UUID              `json:"id"`
	URL       string                 `json:"url"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
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

// GetPostResponse represents the response for getting a single post
type GetPostResponse struct {
	Post *Post `json:"post"`
}

// FeedResponse represents the paginated feed response
type FeedResponse struct {
	Posts      []*Post `json:"posts"`
	HasMore    bool    `json:"has_more"`
	NextCursor *string `json:"next_cursor,omitempty"`
}

// DeletePostResponse represents the response for deleting a post
type DeletePostResponse struct {
	Post    *Post  `json:"post"`
	Message string `json:"message"`
}

// RestorePostResponse represents the response for restoring a post
type RestorePostResponse struct {
	Post Post `json:"post"`
}

// JSONMap is a custom type for storing JSON metadata
type JSONMap map[string]interface{}

// Value implements the driver.Valuer interface
func (j JSONMap) Value() (driver.Value, error) {
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface
func (j *JSONMap) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion failed")
	}
	return json.Unmarshal(bytes, &j)
}
