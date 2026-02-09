package models

import (
	"time"

	"github.com/google/uuid"
)

// WatchLog represents a user's watched movie entry for a post.
type WatchLog struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	PostID    uuid.UUID  `json:"post_id"`
	Rating    int        `json:"rating"`
	Notes     *string    `json:"notes,omitempty"`
	WatchedAt time.Time  `json:"watched_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// LogWatchRequest represents the request body for logging a watch.
type LogWatchRequest struct {
	Rating    int       `json:"rating"`
	Notes     *string   `json:"notes,omitempty"`
	WatchedAt time.Time `json:"watched_at"`
}

// UpdateWatchLogRequest represents the request body for updating a watch log.
type UpdateWatchLogRequest struct {
	Rating *int    `json:"rating,omitempty"`
	Notes  *string `json:"notes,omitempty"`
}

// CreateWatchLogResponse represents the response for creating a watch log.
type CreateWatchLogResponse struct {
	WatchLog WatchLog `json:"watch_log"`
}

// UpdateWatchLogResponse represents the response for updating a watch log.
type UpdateWatchLogResponse struct {
	WatchLog WatchLog `json:"watch_log"`
}

// WatchLogUser represents user information attached to a watch log response.
type WatchLogUser struct {
	ID                uuid.UUID `json:"id"`
	Username          string    `json:"username"`
	ProfilePictureUrl *string   `json:"profile_picture_url,omitempty"`
}

// WatchLogResponse represents a single watch log with user information.
type WatchLogResponse struct {
	WatchLog WatchLog     `json:"watch_log"`
	User     WatchLogUser `json:"user"`
}

// WatchLogWithPost represents a watch log with its related post.
type WatchLogWithPost struct {
	WatchLog
	Post *Post `json:"post,omitempty"`
}

// PostWatchLogsResponse represents watch log summary and entries for a post.
type PostWatchLogsResponse struct {
	WatchCount    int                `json:"watch_count"`
	AvgRating     *float64           `json:"avg_rating,omitempty"`
	Logs          []WatchLogResponse `json:"logs"`
	ViewerWatched bool               `json:"viewer_watched"`
	ViewerRating  *int               `json:"viewer_rating,omitempty"`
}

// ListWatchLogsResponse represents the response for listing a user's watch logs.
type ListWatchLogsResponse struct {
	WatchLogs  []WatchLogWithPost `json:"watch_logs"`
	NextCursor *string            `json:"next_cursor,omitempty"`
}
