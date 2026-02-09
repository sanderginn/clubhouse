package models

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
	AverageRating     float64  `json:"average_rating"`
	ViewerOnBookshelf bool     `json:"viewer_on_bookshelf"`
	ViewerCategories  []string `json:"viewer_categories,omitempty"`
	ViewerRead        bool     `json:"viewer_read"`
	ViewerRating      *int     `json:"viewer_rating,omitempty"`
}

type BookshelfCategoryGroup struct {
	Name  string `json:"name"`
	Posts []Post `json:"posts"`
}

// AddToBookshelfRequest represents the request body for adding a post to a bookshelf.
type AddToBookshelfRequest struct {
	Categories []string `json:"categories,omitempty"`
}

// BookshelfResponse represents bookshelf items grouped by category.
type BookshelfResponse struct {
	Categories []BookshelfCategoryGroup `json:"categories"`
}

// PostBookshelfInfo represents bookshelf tooltip data for a post.
type PostBookshelfInfo struct {
	SaveCount        int            `json:"save_count"`
	Users            []ReactionUser `json:"users"`
	ViewerSaved      bool           `json:"viewer_saved"`
	ViewerCategories []string       `json:"viewer_categories,omitempty"`
}
