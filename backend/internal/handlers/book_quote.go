package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

const (
	defaultBookQuoteListLimit = 20
	maxBookQuoteListLimit     = 100
)

// BookQuoteHandler handles book quote endpoints.
type BookQuoteHandler struct {
	bookQuoteService *services.BookQuoteService
}

// NewBookQuoteHandler creates a new book quote handler.
func NewBookQuoteHandler(bookQuoteService *services.BookQuoteService) *BookQuoteHandler {
	return &BookQuoteHandler{
		bookQuoteService: bookQuoteService,
	}
}

// CreateQuote handles POST /api/v1/posts/{id}/quotes.
func (h *BookQuoteHandler) CreateQuote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	postID, err := extractPostIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	var req models.CreateBookQuoteRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if strings.TrimSpace(req.QuoteText) == "" {
		writeError(r.Context(), w, http.StatusBadRequest, "QUOTE_TEXT_REQUIRED", "quote text is required")
		return
	}

	quote, err := h.bookQuoteService.CreateQuote(r.Context(), userID, postID, req)
	if err != nil {
		switch err.Error() {
		case "quote text is required":
			writeError(r.Context(), w, http.StatusBadRequest, "QUOTE_TEXT_REQUIRED", err.Error())
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		case "post is not a book":
			writeError(r.Context(), w, http.StatusBadRequest, "POST_NOT_BOOK", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "BOOK_QUOTE_CREATE_FAILED", "Failed to create book quote")
		}
		return
	}

	observability.LogInfo(r.Context(), "book quote created",
		"user_id", userID.String(),
		"post_id", postID.String(),
		"quote_id", quote.ID.String(),
	)

	response := models.BookQuoteResponse{Quote: *quote}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode create book quote response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusCreated,
			Err:        err,
		})
	}
}

// GetPostQuotes handles GET /api/v1/posts/{id}/quotes.
func (h *BookQuoteHandler) GetPostQuotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	if _, err := middleware.GetUserIDFromContext(r.Context()); err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	postID, err := extractPostIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	cursor, limit, err := parseBookQuoteListQuery(r)
	if err != nil {
		if err.Error() == "limit must be a positive integer" {
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_LIMIT", err.Error())
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	response, err := h.bookQuoteService.GetQuotesForPost(r.Context(), postID, cursor, limit)
	if err != nil {
		switch err.Error() {
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		case "post is not a book":
			writeError(r.Context(), w, http.StatusBadRequest, "POST_NOT_BOOK", err.Error())
		case "invalid cursor":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_CURSOR", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "GET_BOOK_QUOTES_FAILED", "Failed to list book quotes")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode get post book quotes response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// UpdateQuote handles PUT /api/v1/quotes/{id}.
func (h *BookQuoteHandler) UpdateQuote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PUT requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	quoteID, err := extractQuoteIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_QUOTE_ID", "Invalid quote ID format")
		return
	}

	var req models.UpdateBookQuoteRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if req.QuoteText != nil && strings.TrimSpace(*req.QuoteText) == "" {
		writeError(r.Context(), w, http.StatusBadRequest, "QUOTE_TEXT_REQUIRED", "quote text is required")
		return
	}

	quote, err := h.bookQuoteService.UpdateQuote(r.Context(), userID, quoteID, req)
	if err != nil {
		switch err.Error() {
		case "book quote not found":
			writeError(r.Context(), w, http.StatusNotFound, "BOOK_QUOTE_NOT_FOUND", err.Error())
		case "unauthorized to edit this quote":
			writeError(r.Context(), w, http.StatusForbidden, "FORBIDDEN", "You can only edit your own quotes")
		case "quote text is required":
			writeError(r.Context(), w, http.StatusBadRequest, "QUOTE_TEXT_REQUIRED", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "BOOK_QUOTE_UPDATE_FAILED", "Failed to update book quote")
		}
		return
	}

	observability.LogInfo(r.Context(), "book quote updated",
		"user_id", userID.String(),
		"quote_id", quoteID.String(),
	)

	response := models.BookQuoteResponse{Quote: *quote}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode update book quote response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// DeleteQuote handles DELETE /api/v1/quotes/{id}.
func (h *BookQuoteHandler) DeleteQuote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	quoteID, err := extractQuoteIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_QUOTE_ID", "Invalid quote ID format")
		return
	}

	if err := h.bookQuoteService.DeleteQuote(r.Context(), userID, quoteID); err != nil {
		switch err.Error() {
		case "book quote not found":
			writeError(r.Context(), w, http.StatusNotFound, "BOOK_QUOTE_NOT_FOUND", err.Error())
		case "unauthorized to delete this quote":
			writeError(r.Context(), w, http.StatusForbidden, "FORBIDDEN", "You can only delete your own quotes")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "BOOK_QUOTE_DELETE_FAILED", "Failed to delete book quote")
		}
		return
	}

	observability.LogInfo(r.Context(), "book quote deleted",
		"user_id", userID.String(),
		"quote_id", quoteID.String(),
	)

	w.WriteHeader(http.StatusNoContent)
}

// GetUserQuotes handles GET /api/v1/users/{id}/quotes.
func (h *BookQuoteHandler) GetUserQuotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	if _, err := middleware.GetUserIDFromContext(r.Context()); err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	targetUserID, err := extractBookQuoteUserIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID format")
		return
	}

	cursor, limit, err := parseBookQuoteListQuery(r)
	if err != nil {
		if err.Error() == "limit must be a positive integer" {
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_LIMIT", err.Error())
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	response, err := h.bookQuoteService.GetQuotesByUser(r.Context(), targetUserID, cursor, limit)
	if err != nil {
		switch err.Error() {
		case "invalid cursor":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_CURSOR", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "GET_BOOK_QUOTES_FAILED", "Failed to list book quotes")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode get user book quotes response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

func parseBookQuoteListQuery(r *http.Request) (*string, int, error) {
	limit := defaultBookQuoteListLimit
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			return nil, 0, errors.New("limit must be a positive integer")
		}
		limit = parsed
	}
	if limit > maxBookQuoteListLimit {
		limit = maxBookQuoteListLimit
	}

	var cursor *string
	if cursorParam := strings.TrimSpace(r.URL.Query().Get("cursor")); cursorParam != "" {
		cursor = &cursorParam
	}

	return cursor, limit, nil
}

func extractQuoteIDFromPath(path string) (uuid.UUID, error) {
	pathParts := strings.Split(path, "/")
	for i, part := range pathParts {
		if part == "quotes" && i+1 < len(pathParts) {
			return uuid.Parse(pathParts[i+1])
		}
	}

	return uuid.Nil, errors.New("quote ID not found in path")
}

func extractBookQuoteUserIDFromPath(path string) (uuid.UUID, error) {
	pathParts := strings.Split(path, "/")
	for i, part := range pathParts {
		if part == "users" && i+1 < len(pathParts) {
			return uuid.Parse(pathParts[i+1])
		}
	}

	return uuid.Nil, errors.New("user ID not found in path")
}
