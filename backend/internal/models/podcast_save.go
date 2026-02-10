package models

import (
	"time"

	"github.com/google/uuid"
)

// PodcastSave stores a user's save-for-later state for a podcast post.
type PodcastSave struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	PostID    uuid.UUID  `json:"post_id"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// PostPodcastSaveInfo represents save tooltip data for a podcast post.
type PostPodcastSaveInfo struct {
	SaveCount   int            `json:"save_count"`
	Users       []ReactionUser `json:"users"`
	ViewerSaved bool           `json:"viewer_saved"`
}
