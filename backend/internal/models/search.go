package models

import "github.com/google/uuid"

// LinkMetadataResult represents a link metadata search hit.
type LinkMetadataResult struct {
	ID        uuid.UUID              `json:"id"`
	URL       string                 `json:"url"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	PostID    *uuid.UUID             `json:"post_id,omitempty"`
	CommentID *uuid.UUID             `json:"comment_id,omitempty"`
}

// SearchResult represents a single search hit.
type SearchResult struct {
	Type         string              `json:"type"`
	Score        float64             `json:"score"`
	Post         *Post               `json:"post,omitempty"`
	Comment      *Comment            `json:"comment,omitempty"`
	LinkMetadata *LinkMetadataResult `json:"link_metadata,omitempty"`
}

// SearchResponse represents the response for search requests.
type SearchResponse struct {
	Results []SearchResult `json:"results"`
}
