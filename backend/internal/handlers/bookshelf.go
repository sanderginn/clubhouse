package handlers

import (
	"database/sql"
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
	defaultBookshelfListLimit = 20
	maxBookshelfListLimit     = 100
)

// BookshelfHandler handles bookshelf endpoints.
type BookshelfHandler struct {
	bookshelfService *services.BookshelfService
}

// NewBookshelfHandler creates a new bookshelf handler.
func NewBookshelfHandler(bookshelfService *services.BookshelfService) *BookshelfHandler {
	return &BookshelfHandler{
		bookshelfService: bookshelfService,
	}
}

// CreateCategory handles POST /api/v1/bookshelf/categories.
func (h *BookshelfHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	var req models.CreateBookshelfCategoryRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_REQUIRED", "Category name is required")
		return
	}

	category, err := h.bookshelfService.CreateCategory(r.Context(), userID, req.Name)
	if err != nil {
		switch err.Error() {
		case "category name is required":
			writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_REQUIRED", err.Error())
		case "category name is reserved":
			writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_RESERVED", err.Error())
		case "category name must be 100 characters or less":
			writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_TOO_LONG", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "CREATE_BOOKSHELF_CATEGORY_FAILED", "Failed to create bookshelf category")
		}
		return
	}

	observability.LogInfo(r.Context(), "bookshelf category created",
		"user_id", userID.String(),
		"category_id", category.ID.String(),
	)

	response := models.CreateBookshelfCategoryResponse{Category: *category}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode create bookshelf category response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusCreated,
			Err:        err,
		})
	}
}

// ListCategories handles GET /api/v1/bookshelf/categories.
func (h *BookshelfHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	categories, err := h.bookshelfService.GetCategories(r.Context(), userID)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_BOOKSHELF_CATEGORIES_FAILED", "Failed to get bookshelf categories")
		return
	}

	response := models.ListBookshelfCategoriesResponse{Categories: categories}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode list bookshelf categories response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// UpdateCategory handles PUT /api/v1/bookshelf/categories/{id}.
func (h *BookshelfHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PUT requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	categoryID, err := extractBookshelfCategoryIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_CATEGORY_ID", "Invalid category ID format")
		return
	}

	var req models.UpdateBookshelfCategoryRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_REQUIRED", "Category name is required")
		return
	}
	if req.Position < 0 {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_POSITION", "position must be greater than or equal to 0")
		return
	}

	category, err := h.bookshelfService.UpdateCategory(r.Context(), userID, categoryID, req)
	if err != nil {
		switch err.Error() {
		case "category not found":
			writeError(r.Context(), w, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
		case "category name is required":
			writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_REQUIRED", err.Error())
		case "category name is reserved":
			writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_RESERVED", err.Error())
		case "category name must be 100 characters or less":
			writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_TOO_LONG", err.Error())
		case "position must be greater than or equal to 0":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_POSITION", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "UPDATE_BOOKSHELF_CATEGORY_FAILED", "Failed to update bookshelf category")
		}
		return
	}

	observability.LogInfo(r.Context(), "bookshelf category updated",
		"user_id", userID.String(),
		"category_id", categoryID.String(),
	)

	response := models.UpdateBookshelfCategoryResponse{Category: *category}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode update bookshelf category response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// DeleteCategory handles DELETE /api/v1/bookshelf/categories/{id}.
func (h *BookshelfHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	categoryID, err := extractBookshelfCategoryIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_CATEGORY_ID", "Invalid category ID format")
		return
	}

	if err := h.bookshelfService.DeleteCategory(r.Context(), userID, categoryID); err != nil {
		if err.Error() == "category not found" {
			writeError(r.Context(), w, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "DELETE_BOOKSHELF_CATEGORY_FAILED", "Failed to delete bookshelf category")
		return
	}

	observability.LogInfo(r.Context(), "bookshelf category deleted",
		"user_id", userID.String(),
		"category_id", categoryID.String(),
	)

	w.WriteHeader(http.StatusNoContent)
}

// ReorderCategories handles POST /api/v1/bookshelf/categories/reorder.
func (h *BookshelfHandler) ReorderCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	var req models.ReorderBookshelfCategoriesRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	if len(req.CategoryIDs) == 0 {
		writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_IDS_REQUIRED", "category_ids must not be empty")
		return
	}

	if err := h.bookshelfService.ReorderCategories(r.Context(), userID, req.CategoryIDs); err != nil {
		switch err.Error() {
		case "category_ids must not be empty":
			writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_IDS_REQUIRED", err.Error())
		case "duplicate category id":
			writeError(r.Context(), w, http.StatusBadRequest, "DUPLICATE_CATEGORY_ID", err.Error())
		case "category_ids must include all user categories":
			writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_IDS_MISMATCH", err.Error())
		case "category not found":
			writeError(r.Context(), w, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "REORDER_BOOKSHELF_CATEGORIES_FAILED", "Failed to reorder bookshelf categories")
		}
		return
	}

	observability.LogInfo(r.Context(), "bookshelf categories reordered",
		"user_id", userID.String(),
		"category_count", strconv.Itoa(len(req.CategoryIDs)),
	)

	w.WriteHeader(http.StatusNoContent)
}

// AddToBookshelf handles POST /api/v1/posts/{postId}/bookshelf.
func (h *BookshelfHandler) AddToBookshelf(w http.ResponseWriter, r *http.Request) {
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

	var req models.AddToBookshelfRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := h.bookshelfService.AddToBookshelf(r.Context(), userID, postID, req.Categories); err != nil {
		switch err.Error() {
		case "book post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
		case "category name must be 100 characters or less":
			writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_TOO_LONG", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "BOOKSHELF_ADD_FAILED", "Failed to add post to bookshelf")
		}
		return
	}

	observability.LogInfo(r.Context(), "post added to bookshelf",
		"user_id", userID.String(),
		"post_id", postID.String(),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(struct{}{}); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode add to bookshelf response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusCreated,
			Err:        err,
		})
	}
}

// RemoveFromBookshelf handles DELETE /api/v1/posts/{postId}/bookshelf.
func (h *BookshelfHandler) RemoveFromBookshelf(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
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

	if err := h.bookshelfService.RemoveFromBookshelf(r.Context(), userID, postID); err != nil {
		switch err.Error() {
		case "book post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
		case "bookshelf item not found":
			writeError(r.Context(), w, http.StatusNotFound, "BOOKSHELF_ITEM_NOT_FOUND", "Bookshelf item not found")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "BOOKSHELF_REMOVE_FAILED", "Failed to remove post from bookshelf")
		}
		return
	}

	observability.LogInfo(r.Context(), "post removed from bookshelf",
		"user_id", userID.String(),
		"post_id", postID.String(),
	)

	w.WriteHeader(http.StatusNoContent)
}

// GetMyBookshelf handles GET /api/v1/bookshelf.
func (h *BookshelfHandler) GetMyBookshelf(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	category, cursor, limit, err := parseBookshelfListQuery(r)
	if err != nil {
		if err.Error() == "limit must be a positive integer" {
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_LIMIT", err.Error())
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	items, nextCursor, err := h.bookshelfService.GetUserBookshelf(r.Context(), userID, category, cursor, limit)
	if err != nil {
		switch err.Error() {
		case "invalid cursor":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_CURSOR", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "GET_BOOKSHELF_FAILED", "Failed to get bookshelf items")
		}
		return
	}

	response := models.ListBookshelfItemsResponse{
		BookshelfItems: items,
		NextCursor:     nextCursor,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode get bookshelf response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// GetAllBookshelf handles GET /api/v1/bookshelf/all.
func (h *BookshelfHandler) GetAllBookshelf(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	_, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	category, cursor, limit, err := parseBookshelfListQuery(r)
	if err != nil {
		if err.Error() == "limit must be a positive integer" {
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_LIMIT", err.Error())
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	items, nextCursor, err := h.bookshelfService.GetAllBookshelfItems(r.Context(), category, cursor, limit)
	if err != nil {
		switch err.Error() {
		case "invalid cursor":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_CURSOR", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "GET_ALL_BOOKSHELF_FAILED", "Failed to get bookshelf items")
		}
		return
	}

	response := models.ListBookshelfItemsResponse{
		BookshelfItems: items,
		NextCursor:     nextCursor,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode get all bookshelf response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

func parseBookshelfListQuery(r *http.Request) (*string, *string, int, error) {
	limit := defaultBookshelfListLimit
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			return nil, nil, 0, errors.New("limit must be a positive integer")
		}
		limit = parsed
	}
	if limit > maxBookshelfListLimit {
		limit = maxBookshelfListLimit
	}

	var category *string
	if categoryParam := strings.TrimSpace(r.URL.Query().Get("category")); categoryParam != "" {
		category = &categoryParam
	}

	var cursor *string
	if cursorParam := strings.TrimSpace(r.URL.Query().Get("cursor")); cursorParam != "" {
		cursor = &cursorParam
	}

	return category, cursor, limit, nil
}

func extractBookshelfCategoryIDFromPath(path string) (uuid.UUID, error) {
	pathParts := strings.Split(path, "/")
	for i, part := range pathParts {
		if part == "categories" && i+1 < len(pathParts) {
			return uuid.Parse(pathParts[i+1])
		}
	}
	return uuid.Nil, sql.ErrNoRows
}
