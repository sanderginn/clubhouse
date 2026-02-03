package models

import (
	"time"

	"github.com/google/uuid"
)

type CookLog struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	PostID    uuid.UUID  `json:"post_id"`
	Rating    int        `json:"rating"`
	Notes     *string    `json:"notes,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// CookLogUser represents a minimal user payload with rating info for cook tooltips.
type CookLogUser struct {
	ID                uuid.UUID `json:"id"`
	Username          string    `json:"username"`
	ProfilePictureUrl *string   `json:"profile_picture_url,omitempty"`
	Rating            int       `json:"rating"`
	CreatedAt         time.Time `json:"created_at"`
}

// PostCookInfo represents cook tooltip data for a post.
type PostCookInfo struct {
	CookCount     int           `json:"cook_count"`
	AvgRating     *float64      `json:"avg_rating,omitempty"`
	Users         []CookLogUser `json:"users"`
	ViewerCooked  bool          `json:"viewer_cooked"`
	ViewerCookLog *CookLog      `json:"viewer_cook_log,omitempty"`
}

type CookLogWithPost struct {
	CookLog
	Post *Post `json:"post,omitempty"`
}

// CreateCookLogRequest represents the request body for creating a cook log.
type CreateCookLogRequest struct {
	PostID string  `json:"post_id"`
	Rating int     `json:"rating"`
	Notes  *string `json:"notes,omitempty"`
}

// CreateCookLogResponse represents the response for creating a cook log.
type CreateCookLogResponse struct {
	CookLog CookLog `json:"cook_log"`
}

// UpdateCookLogRequest represents the request body for updating a cook log.
type UpdateCookLogRequest struct {
	Rating *int    `json:"rating,omitempty"`
	Notes  *string `json:"notes,omitempty"`
}

// UpdateCookLogResponse represents the response for updating a cook log.
type UpdateCookLogResponse struct {
	CookLog CookLog `json:"cook_log"`
}

// DeleteCookLogResponse represents the response for deleting a cook log.
type DeleteCookLogResponse struct {
	CookLog *CookLog `json:"cook_log"`
	Message string   `json:"message"`
}

// GetPostCookInfoResponse represents the response for cook tooltip data.
type GetPostCookInfoResponse struct {
	CookInfo PostCookInfo `json:"cook_info"`
}

// ListCookLogsResponse represents the response for listing cook logs.
type ListCookLogsResponse struct {
	CookLogs []CookLogWithPost `json:"cook_logs"`
}
