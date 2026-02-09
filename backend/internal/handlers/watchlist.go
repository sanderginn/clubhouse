package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

// WatchlistHandler handles movie/series watchlist endpoints.
type WatchlistHandler struct {
	watchlistService *services.WatchlistService
	postService      *services.PostService
	userService      *services.UserService
	redis            *redis.Client
}

// NewWatchlistHandler creates a new watchlist handler.
func NewWatchlistHandler(db *sql.DB, redisClient *redis.Client) *WatchlistHandler {
	return &WatchlistHandler{
		watchlistService: services.NewWatchlistService(db),
		postService:      services.NewPostService(db),
		userService:      services.NewUserService(db),
		redis:            redisClient,
	}
}

// AddToWatchlist handles POST /api/v1/posts/{postId}/watchlist.
func (h *WatchlistHandler) AddToWatchlist(w http.ResponseWriter, r *http.Request) {
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

	var req models.AddToWatchlistRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	items, err := h.watchlistService.AddToWatchlist(r.Context(), userID, postID, req.Categories)
	if err != nil {
		switch err.Error() {
		case "movie or series post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
		case "category not found":
			writeError(r.Context(), w, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
		case "category name must be 100 characters or less":
			writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_TOO_LONG", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "WATCHLIST_ADD_FAILED", "Failed to add to watchlist")
		}
		return
	}

	categories := uniqueWatchlistCategories(items)
	publishCtx, cancel := publishContext()
	username := ""
	if user, err := h.userService.GetUserByID(publishCtx, userID); err == nil {
		username = user.Username
	} else {
		observability.LogWarn(publishCtx, "failed to load user for movie_watchlisted event",
			"user_id", userID.String(),
			"post_id", postID.String(),
			"error", err.Error(),
		)
	}
	eventData := movieWatchlistedEventData{
		PostID:     postID,
		UserID:     userID,
		Username:   username,
		Categories: categories,
	}
	if sectionID, err := h.postService.GetSectionIDByPostID(publishCtx, postID); err == nil {
		_ = publishEvent(publishCtx, h.redis, formatChannel(sectionPrefix, sectionID), "movie_watchlisted", eventData)
	}
	cancel()

	observability.LogInfo(r.Context(), "movie watchlisted",
		"user_id", userID.String(),
		"post_id", postID.String(),
		"category_count", strconv.Itoa(len(items)),
	)

	response := models.AddToWatchlistResponse{
		WatchlistItems: items,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode add to watchlist response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// RemoveFromWatchlist handles DELETE /api/v1/posts/{postId}/watchlist.
func (h *WatchlistHandler) RemoveFromWatchlist(w http.ResponseWriter, r *http.Request) {
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

	var category *string
	if categoryParam := strings.TrimSpace(r.URL.Query().Get("category")); categoryParam != "" {
		category = &categoryParam
	}

	if err := h.watchlistService.RemoveFromWatchlist(r.Context(), userID, postID, category); err != nil {
		switch err.Error() {
		case "movie or series post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
			return
		case "category name must be 100 characters or less":
			writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_TOO_LONG", err.Error())
			return
		case "watchlist item not found":
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "WATCHLIST_REMOVE_FAILED", "Failed to remove from watchlist")
			return
		}
	}

	publishCtx, cancel := publishContext()
	eventData := movieUnwatchlistedEventData{
		PostID: postID,
		UserID: userID,
	}
	if sectionID, err := h.postService.GetSectionIDByPostID(publishCtx, postID); err == nil {
		_ = publishEvent(publishCtx, h.redis, formatChannel(sectionPrefix, sectionID), "movie_unwatchlisted", eventData)
	}
	cancel()

	observability.LogInfo(r.Context(), "movie unwatchlisted",
		"user_id", userID.String(),
		"post_id", postID.String(),
	)

	w.WriteHeader(http.StatusNoContent)
}

// GetPostWatchlistInfo handles GET /api/v1/posts/{postId}/watchlist-info.
func (h *WatchlistHandler) GetPostWatchlistInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
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

	info, err := h.watchlistService.GetPostWatchlistInfo(r.Context(), postID, &userID)
	if err != nil {
		if err.Error() == "movie or series post not found" {
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_WATCHLIST_INFO_FAILED", "Failed to get watchlist info")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode post watchlist info response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// ListWatchlist handles GET /api/v1/me/watchlist.
func (h *WatchlistHandler) ListWatchlist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	grouped, err := h.watchlistService.GetUserWatchlist(r.Context(), userID)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_WATCHLIST_FAILED", "Failed to get watchlist")
		return
	}

	response := models.WatchlistResponse{
		Categories: buildWatchlistCategoryGroups(grouped),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode watchlist response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// ListWatchlistCategories handles GET /api/v1/me/watchlist-categories.
func (h *WatchlistHandler) ListWatchlistCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	categories, err := h.watchlistService.GetUserWatchlistCategories(r.Context(), userID)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_WATCHLIST_CATEGORIES_FAILED", "Failed to get watchlist categories")
		return
	}

	response := models.ListWatchlistCategoriesResponse{
		Categories: categories,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode watchlist categories response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// CreateWatchlistCategory handles POST /api/v1/me/watchlist-categories.
func (h *WatchlistHandler) CreateWatchlistCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	var req models.CreateWatchlistCategoryRequest
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

	category, err := h.watchlistService.CreateCategory(r.Context(), userID, req.Name)
	if err != nil {
		switch err.Error() {
		case "category name must be 100 characters or less":
			writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_TOO_LONG", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "CREATE_WATCHLIST_CATEGORY_FAILED", "Failed to create watchlist category")
		}
		return
	}

	observability.LogInfo(r.Context(), "watchlist category created",
		"user_id", userID.String(),
		"category_id", category.ID.String(),
	)

	response := models.CreateWatchlistCategoryResponse{
		Category: *category,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode create watchlist category response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusCreated,
			Err:        err,
		})
	}
}

// UpdateWatchlistCategory handles PATCH /api/v1/me/watchlist-categories/{id}.
func (h *WatchlistHandler) UpdateWatchlistCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PATCH requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	categoryID, err := extractWatchlistCategoryIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_CATEGORY_ID", "Invalid category ID format")
		return
	}

	var req models.UpdateWatchlistCategoryRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_REQUIRED", "Category name is required")
		return
	}

	if err := h.watchlistService.UpdateCategory(r.Context(), userID, categoryID, req.Name, req.Position); err != nil {
		switch err.Error() {
		case "category not found":
			writeError(r.Context(), w, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
		case "category name must be 100 characters or less":
			writeError(r.Context(), w, http.StatusBadRequest, "CATEGORY_NAME_TOO_LONG", err.Error())
		case "no updates provided":
			writeError(r.Context(), w, http.StatusBadRequest, "NO_UPDATES", "No updates provided")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "UPDATE_WATCHLIST_CATEGORY_FAILED", "Failed to update watchlist category")
		}
		return
	}

	updated, err := h.fetchUserWatchlistCategory(r.Context(), userID, categoryID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(r.Context(), w, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "UPDATE_WATCHLIST_CATEGORY_FAILED", "Failed to load updated category")
		return
	}

	observability.LogInfo(r.Context(), "watchlist category updated",
		"user_id", userID.String(),
		"category_id", categoryID.String(),
	)

	response := models.UpdateWatchlistCategoryResponse{
		Category: *updated,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode update watchlist category response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// DeleteWatchlistCategory handles DELETE /api/v1/me/watchlist-categories/{id}.
func (h *WatchlistHandler) DeleteWatchlistCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	categoryID, err := extractWatchlistCategoryIDFromPath(r.URL.Path)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_CATEGORY_ID", "Invalid category ID format")
		return
	}

	if err := h.watchlistService.DeleteCategory(r.Context(), userID, categoryID); err != nil {
		if err.Error() == "category not found" {
			writeError(r.Context(), w, http.StatusNotFound, "CATEGORY_NOT_FOUND", "Category not found")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "DELETE_WATCHLIST_CATEGORY_FAILED", "Failed to delete watchlist category")
		return
	}

	observability.LogInfo(r.Context(), "watchlist category deleted",
		"user_id", userID.String(),
		"category_id", categoryID.String(),
	)

	w.WriteHeader(http.StatusNoContent)
}

func (h *WatchlistHandler) fetchUserWatchlistCategory(ctx context.Context, userID, categoryID uuid.UUID) (*models.WatchlistCategory, error) {
	categories, err := h.watchlistService.GetUserWatchlistCategories(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, category := range categories {
		if category.ID == categoryID {
			return &category, nil
		}
	}
	return nil, sql.ErrNoRows
}

func extractWatchlistCategoryIDFromPath(path string) (uuid.UUID, error) {
	pathParts := strings.Split(path, "/")
	for i, part := range pathParts {
		if part == "watchlist-categories" && i+1 < len(pathParts) {
			return uuid.Parse(pathParts[i+1])
		}
	}
	return uuid.Nil, sql.ErrNoRows
}

func buildWatchlistCategoryGroups(grouped map[string][]models.WatchlistItemWithPost) []models.WatchlistCategoryGroup {
	if len(grouped) == 0 {
		return []models.WatchlistCategoryGroup{}
	}

	names := make([]string, 0, len(grouped))
	for name := range grouped {
		names = append(names, name)
	}
	sort.Strings(names)

	categories := make([]models.WatchlistCategoryGroup, 0, len(names))
	for _, name := range names {
		categories = append(categories, models.WatchlistCategoryGroup{
			Name:  name,
			Items: grouped[name],
		})
	}

	return categories
}

func uniqueWatchlistCategories(items []models.WatchlistItem) []string {
	if len(items) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(items))
	categories := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item.Category]; ok {
			continue
		}
		seen[item.Category] = struct{}{}
		categories = append(categories, item.Category)
	}
	return categories
}
