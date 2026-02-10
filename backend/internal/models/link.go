package models

import (
	"time"

	"github.com/google/uuid"
)

// SectionLink represents a link with post and user context for section aggregation.
type SectionLink struct {
	ID        uuid.UUID              `json:"id"`
	URL       string                 `json:"url"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	PostID    uuid.UUID              `json:"post_id"`
	UserID    uuid.UUID              `json:"user_id"`
	Username  string                 `json:"username"`
	CreatedAt time.Time              `json:"created_at"`
}

// SectionLinksResponse represents a paginated response for section links.
type SectionLinksResponse struct {
	Links      []SectionLink `json:"links"`
	HasMore    bool          `json:"has_more"`
	NextCursor *string       `json:"next_cursor,omitempty"`
}

// RecentPodcastItem represents a recently shared podcast link in a section.
type RecentPodcastItem struct {
	PostID        uuid.UUID       `json:"post_id"`
	LinkID        uuid.UUID       `json:"link_id"`
	URL           string          `json:"url"`
	Podcast       PodcastMetadata `json:"podcast"`
	UserID        uuid.UUID       `json:"user_id"`
	Username      string          `json:"username"`
	PostCreatedAt time.Time       `json:"post_created_at"`
	LinkCreatedAt time.Time       `json:"link_created_at"`
}

// SectionRecentPodcastsResponse represents a paginated response for recent podcast items.
type SectionRecentPodcastsResponse struct {
	Items      []RecentPodcastItem `json:"items"`
	HasMore    bool                `json:"has_more"`
	NextCursor *string             `json:"next_cursor,omitempty"`
}
