package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Post represents a post in the system
type Post struct {
	ID              uuid.UUID      `json:"id"`
	UserID          uuid.UUID      `json:"user_id"`
	SectionID       uuid.UUID      `json:"section_id"`
	Content         string         `json:"content"`
	Links           []Link         `json:"links,omitempty"`
	Images          []PostImage    `json:"images,omitempty"`
	CommentCount    int            `json:"comment_count"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       *time.Time     `json:"updated_at,omitempty"`
	DeletedAt       *time.Time     `json:"deleted_at,omitempty"`
	DeletedByUserID *uuid.UUID     `json:"deleted_by_user_id,omitempty"`
	User            *User          `json:"user,omitempty"`
	ReactionCounts  map[string]int `json:"reaction_counts,omitempty"`
	ViewerReactions []string       `json:"viewer_reactions,omitempty"`
	RecipeStats     *RecipeStats   `json:"recipe_stats,omitempty"`
}

type RecipeStats struct {
	SaveCount        int      `json:"save_count"`
	CookCount        int      `json:"cook_count"`
	AvgRating        *float64 `json:"avg_rating,omitempty"`
	ViewerSaved      bool     `json:"viewer_saved"`
	ViewerCooked     bool     `json:"viewer_cooked"`
	ViewerCategories []string `json:"viewer_categories,omitempty"`
}

// Link represents metadata for a URL
type Link struct {
	ID         uuid.UUID              `json:"id"`
	URL        string                 `json:"url"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Highlights []Highlight            `json:"highlights,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

// PostImage represents an image attached to a post.
type PostImage struct {
	ID        uuid.UUID `json:"id"`
	URL       string    `json:"url"`
	Position  int       `json:"position"`
	Caption   *string   `json:"caption,omitempty"`
	AltText   *string   `json:"alt_text,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// CreatePostRequest represents the request body for creating a post
type CreatePostRequest struct {
	SectionID string             `json:"section_id"`
	Content   string             `json:"content"`
	Links     []LinkRequest      `json:"links,omitempty"`
	Images    []PostImageRequest `json:"images,omitempty"`
	// MentionUsernames contains explicitly selected mentions from the client.
	MentionUsernames []string `json:"mention_usernames,omitempty"`
}

// LinkRequest represents a link in the request
type LinkRequest struct {
	URL        string      `json:"url"`
	Highlights []Highlight `json:"highlights,omitempty"`
}

// Highlight represents a timestamped highlight for a link.
type Highlight struct {
	Timestamp int    `json:"timestamp"`
	Label     string `json:"label,omitempty"`
}

const (
	maxHighlightsPerLink    = 20
	maxHighlightLabelLength = 100
)

var highlightAllowedSectionTypes = map[string]struct{}{
	"music": {},
}

func ValidateHighlights(sectionType string, highlights []Highlight) error {
	if len(highlights) == 0 {
		return nil
	}

	if _, ok := highlightAllowedSectionTypes[sectionType]; !ok {
		return fmt.Errorf("highlights are not allowed for section type %q", sectionType)
	}

	if len(highlights) > maxHighlightsPerLink {
		return fmt.Errorf("too many highlights")
	}

	for _, highlight := range highlights {
		if highlight.Timestamp < 0 {
			return fmt.Errorf("highlight timestamp must be non-negative")
		}
		if len(highlight.Label) > maxHighlightLabelLength {
			return fmt.Errorf("highlight label must be less than %d characters", maxHighlightLabelLength)
		}
	}

	return nil
}

// PostImageRequest represents an image in the request.
type PostImageRequest struct {
	URL     string  `json:"url"`
	Caption *string `json:"caption,omitempty"`
	AltText *string `json:"alt_text,omitempty"`
}

// UpdatePostRequest represents the request body for updating a post
type UpdatePostRequest struct {
	Content string              `json:"content"`
	Links   *[]LinkRequest      `json:"links,omitempty"`
	Images  *[]PostImageRequest `json:"images,omitempty"`
	// MentionUsernames contains explicitly selected mentions from the client.
	MentionUsernames []string `json:"mention_usernames,omitempty"`
}

// CreatePostResponse represents the response for creating a post
type CreatePostResponse struct {
	Post Post `json:"post"`
}

// GetPostResponse represents the response for getting a single post
type GetPostResponse struct {
	Post *Post `json:"post"`
}

// UpdatePostResponse represents the response for updating a post
type UpdatePostResponse struct {
	Post Post `json:"post"`
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

// HardDeletePostResponse represents the response for permanently deleting a post
type HardDeletePostResponse struct {
	ID      uuid.UUID `json:"id"`
	Message string    `json:"message"`
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
