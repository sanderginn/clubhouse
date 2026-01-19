package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/services"
)

// UserHandler handles user endpoints
type UserHandler struct {
	userService *services.UserService
	postService *services.PostService
}

// NewUserHandler creates a new user handler
func NewUserHandler(db *sql.DB) *UserHandler {
	return &UserHandler{
		userService: services.NewUserService(db),
		postService: services.NewPostService(db),
	}
}

// GetProfile handles GET /api/v1/users/{id}
func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "User ID is required")
		return
	}

	userIDStr := pathParts[4]
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID format")
		return
	}

	profile, err := h.userService.GetUserProfile(r.Context(), userID)
	if err != nil {
		if err.Error() == "user not found" {
			writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "GET_PROFILE_FAILED", "Failed to get user profile")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(profile)
}

// GetUserPosts handles GET /api/v1/users/{id}/posts
func (h *UserHandler) GetUserPosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	// Extract user ID from URL path: /api/v1/users/{id}/posts
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 6 || pathParts[5] != "posts" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "User ID is required")
		return
	}

	userIDStr := pathParts[4]
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID format")
		return
	}

	// Parse query parameters
	cursor := r.URL.Query().Get("cursor")
	limitStr := r.URL.Query().Get("limit")

	limit := 20
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Clamp limit to reasonable range
	if limit > 100 {
		limit = 100
	}

	// Get user posts from service
	var cursorPtr *string
	if cursor != "" {
		cursorPtr = &cursor
	}

	feed, err := h.postService.GetPostsByUserID(r.Context(), userID, cursorPtr, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GET_USER_POSTS_FAILED", "Failed to get user posts")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(feed)
}
