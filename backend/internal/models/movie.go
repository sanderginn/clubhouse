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

// Season represents TV season metadata.
type Season struct {
	SeasonNumber int    `json:"season_number"`
	EpisodeCount int    `json:"episode_count"`
	AirDate      string `json:"air_date"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	Poster       string `json:"poster"`
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
	Seasons     []Season     `json:"seasons,omitempty"`
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

// AddToWatchlistResponse represents the response for adding to watchlist.
type AddToWatchlistResponse struct {
	WatchlistItems []WatchlistItem `json:"watchlist_items"`
}

// WatchlistResponse represents watchlist items grouped by category.
type WatchlistResponse struct {
	Categories []WatchlistCategoryGroup `json:"categories"`
}

// CreateWatchlistCategoryRequest represents the request body for creating a watchlist category.
type CreateWatchlistCategoryRequest struct {
	Name string `json:"name"`
}

// UpdateWatchlistCategoryRequest represents the request body for updating a watchlist category.
type UpdateWatchlistCategoryRequest struct {
	Name     *string `json:"name,omitempty"`
	Position *int    `json:"position,omitempty"`
}

// CreateWatchlistCategoryResponse represents the response for creating a watchlist category.
type CreateWatchlistCategoryResponse struct {
	Category WatchlistCategory `json:"category"`
}

// UpdateWatchlistCategoryResponse represents the response for updating a watchlist category.
type UpdateWatchlistCategoryResponse struct {
	Category WatchlistCategory `json:"category"`
}

// ListWatchlistCategoriesResponse represents the response for listing watchlist categories.
type ListWatchlistCategoriesResponse struct {
	Categories []WatchlistCategory `json:"categories"`
}
