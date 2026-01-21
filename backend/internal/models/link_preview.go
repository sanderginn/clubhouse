package models

// LinkPreviewRequest represents the request body for previewing a link.
type LinkPreviewRequest struct {
	URL string `json:"url"`
}

// LinkPreviewResponse represents the response for link preview metadata.
type LinkPreviewResponse struct {
	Metadata map[string]interface{} `json:"metadata"`
}
