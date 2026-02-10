package models

import (
	"time"

	"github.com/google/uuid"
)

// BookQuote represents a quote saved from a book-related post.
type BookQuote struct {
	ID         uuid.UUID  `json:"id"`
	PostID     uuid.UUID  `json:"post_id"`
	UserID     uuid.UUID  `json:"user_id"`
	QuoteText  string     `json:"quote_text"`
	PageNumber *int       `json:"page_number,omitempty"`
	Chapter    *string    `json:"chapter,omitempty"`
	Note       *string    `json:"note,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

// BookQuoteWithUser represents a book quote with user info for API responses.
type BookQuoteWithUser struct {
	BookQuote
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

// CreateBookQuoteRequest represents the request body for creating a book quote.
type CreateBookQuoteRequest struct {
	QuoteText  string  `json:"quote_text"`
	PageNumber *int    `json:"page_number,omitempty"`
	Chapter    *string `json:"chapter,omitempty"`
	Note       *string `json:"note,omitempty"`
}

// UpdateBookQuoteRequest represents the request body for updating a book quote.
type UpdateBookQuoteRequest struct {
	QuoteText  *string `json:"quote_text,omitempty"`
	PageNumber *int    `json:"page_number,omitempty"`
	Chapter    *string `json:"chapter,omitempty"`
	Note       *string `json:"note,omitempty"`
}

// BookQuoteResponse represents the response for a single book quote.
type BookQuoteResponse struct {
	Quote BookQuoteWithUser `json:"quote"`
}

// BookQuotesListResponse represents a paginated response for book quotes.
type BookQuotesListResponse struct {
	Quotes     []BookQuoteWithUser `json:"quotes"`
	NextCursor *string             `json:"next_cursor,omitempty"`
	HasMore    bool                `json:"has_more"`
}
