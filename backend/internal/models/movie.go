package models

import (
	"time"

	"github.com/google/uuid"
)

// CastMember represents a cast member in movie metadata.
type CastMember struct {
	Name      string `json:"name"`
	Character string `json:"character,omitempty"`
}

// MovieData represents normalized movie metadata.
type MovieData struct {
	Title       string       `json:"title"`
	Overview    string       `json:"overview"`
	Poster      string       `json:"poster"`
	Backdrop    string       `json:"backdrop"`
	Runtime     int          `json:"runtime"`
	Genres      []string     `json:"genres"`
	ReleaseDate string       `json:"release_date"`
	Cast        []CastMember `json:"cast"`
	Director    string       `json:"director"`
	TMDBRating  float64      `json:"tmdb_rating"`
	TrailerKey  string       `json:"trailer_key"`
}

type WatchlistItem struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	PostID    uuid.UUID  `json:"post_id"`
	Category  string     `json:"category"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type WatchlistCategory struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

// PostWatchlistInfo represents watchlist tooltip data for a post.
type PostWatchlistInfo struct {
	SaveCount        int            `json:"save_count"`
	Users            []ReactionUser `json:"users"`
	ViewerSaved      bool           `json:"viewer_saved"`
	ViewerCategories []string       `json:"viewer_categories,omitempty"`
}

type WatchlistItemWithPost struct {
	WatchlistItem
	Post *Post `json:"post,omitempty"`
}

type WatchlistCategoryGroup struct {
	Name  string                  `json:"name"`
	Items []WatchlistItemWithPost `json:"items"`
}

// AddToWatchlistRequest represents the request body for adding a post to a watchlist.
type AddToWatchlistRequest struct {
	Categories []string `json:"categories,omitempty"`
}

// WatchlistResponse represents watchlist items grouped by category.
type WatchlistResponse struct {
	Categories []WatchlistCategoryGroup `json:"categories"`
}
