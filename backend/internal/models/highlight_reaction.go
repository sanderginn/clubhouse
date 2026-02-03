package models

type HighlightReactionResponse struct {
	HighlightID   string `json:"highlight_id"`
	HeartCount    int    `json:"heart_count"`
	ViewerReacted bool   `json:"viewer_reacted"`
}
