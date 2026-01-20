package models

// SearchResult represents a single search hit.
type SearchResult struct {
	Type    string   `json:"type"`
	Score   float64  `json:"score"`
	Post    *Post    `json:"post,omitempty"`
	Comment *Comment `json:"comment,omitempty"`
}

// SearchResponse represents the response for search requests.
type SearchResponse struct {
	Results []SearchResult `json:"results"`
}
