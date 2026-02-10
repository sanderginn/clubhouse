package models

import (
	"time"

	"github.com/google/uuid"
)

// BookData represents normalized book metadata.
type BookData struct {
	Title          string   `json:"title"`
	Authors        []string `json:"authors"`
	Description    string   `json:"description"`
	CoverURL       string   `json:"cover_url"`
	PageCount      int      `json:"page_count"`
	Genres         []string `json:"genres"`
	PublishDate    string   `json:"publish_date"`
	ISBN           string   `json:"isbn"`
	OpenLibraryKey string   `json:"open_library_key"`
	GoodreadsURL   string   `json:"goodreads_url"`
}

// BookStats represents aggregate and viewer-specific reading stats for a post.
type BookStats struct {
	BookshelfCount    int      `json:"bookshelf_count"`
	ReadCount         int      `json:"read_count"`
	RatedCount        int      `json:"rated_count"`
	AverageRating     float64  `json:"average_rating"`
	ViewerOnBookshelf bool     `json:"viewer_on_bookshelf"`
	ViewerCategories  []string `json:"viewer_categories,omitempty"`
	ViewerRead        bool     `json:"viewer_read"`
	ViewerRating      *int     `json:"viewer_rating,omitempty"`
}

type BookshelfItem struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	PostID     uuid.UUID  `json:"post_id"`
	CategoryID *uuid.UUID `json:"category_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

type BookshelfCategory struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

type BookshelfUserInfo struct {
	ID                uuid.UUID `json:"id"`
	Username          string    `json:"username"`
	ProfilePictureUrl *string   `json:"profile_picture_url,omitempty"`
}

type BookshelfCategoryGroup struct {
	Name  string `json:"name"`
	Posts []Post `json:"posts"`
}

// CreateBookshelfCategoryRequest represents the request body for creating a bookshelf category.
type CreateBookshelfCategoryRequest struct {
	Name string `json:"name"`
}

// UpdateBookshelfCategoryRequest represents the request body for updating a bookshelf category.
type UpdateBookshelfCategoryRequest struct {
	Name     string `json:"name"`
	Position int    `json:"position"`
}

// AddToBookshelfRequest represents the request body for adding a post to a bookshelf.
type AddToBookshelfRequest struct {
	Categories []string `json:"categories,omitempty"`
}

// ReorderBookshelfCategoriesRequest represents the request body for reordering categories.
type ReorderBookshelfCategoriesRequest struct {
	CategoryIDs []uuid.UUID `json:"category_ids"`
}

// BookshelfResponse represents bookshelf items grouped by category.
type BookshelfResponse struct {
	Categories []BookshelfCategoryGroup `json:"categories"`
}

// CreateBookshelfCategoryResponse represents the response for creating a bookshelf category.
type CreateBookshelfCategoryResponse struct {
	Category BookshelfCategory `json:"category"`
}

// UpdateBookshelfCategoryResponse represents the response for updating a bookshelf category.
type UpdateBookshelfCategoryResponse struct {
	Category BookshelfCategory `json:"category"`
}

// ListBookshelfCategoriesResponse represents the response for listing bookshelf categories.
type ListBookshelfCategoriesResponse struct {
	Categories []BookshelfCategory `json:"categories"`
}

// ListBookshelfItemsResponse represents a paginated bookshelf item response.
type ListBookshelfItemsResponse struct {
	BookshelfItems []BookshelfItem `json:"bookshelf_items"`
	NextCursor     *string         `json:"next_cursor,omitempty"`
}

// PostBookshelfInfo represents bookshelf tooltip data for a post.
type PostBookshelfInfo struct {
	SaveCount        int                 `json:"save_count"`
	Users            []BookshelfUserInfo `json:"users"`
	ViewerSaved      bool                `json:"viewer_saved"`
	ViewerCategories []string            `json:"viewer_categories,omitempty"`
}

type ReadLog struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	PostID    uuid.UUID  `json:"post_id"`
	Rating    *int       `json:"rating,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type ReadLogUserInfo struct {
	ID                uuid.UUID `json:"id"`
	Username          string    `json:"username"`
	ProfilePictureUrl *string   `json:"profile_picture_url,omitempty"`
	Rating            *int      `json:"rating,omitempty"`
}

type PostReadLogsResponse struct {
	ReadCount     int               `json:"read_count"`
	RatedCount    int               `json:"rated_count"`
	AverageRating float64           `json:"average_rating"`
	ViewerRead    bool              `json:"viewer_read"`
	ViewerRating  *int              `json:"viewer_rating,omitempty"`
	Readers       []ReadLogUserInfo `json:"readers"`
}

// LogReadRequest represents the request body for creating a read log.
type LogReadRequest struct {
	Rating *int `json:"rating,omitempty"`
}

// UpdateReadLogRequest represents the request body for updating a read rating.
type UpdateReadLogRequest struct {
	Rating *int `json:"rating"`
}

// CreateReadLogResponse represents the response for creating a read log.
type CreateReadLogResponse struct {
	ReadLog ReadLog `json:"read_log"`
}

// UpdateReadLogResponse represents the response for updating a read log.
type UpdateReadLogResponse struct {
	ReadLog ReadLog `json:"read_log"`
}

// ListReadHistoryResponse represents paginated read history for a viewer.
type ListReadHistoryResponse struct {
	ReadLogs   []ReadLog `json:"read_logs"`
	NextCursor *string   `json:"next_cursor,omitempty"`
}
