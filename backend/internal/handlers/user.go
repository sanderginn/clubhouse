package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
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

// GetUserComments handles GET /api/v1/users/{id}/comments
func (h *UserHandler) GetUserComments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	// Extract user ID from URL path: /api/v1/users/{id}/comments
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 6 || pathParts[5] != "comments" {
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

	var cursorPtr *string
	if cursor != "" {
		cursorPtr = &cursor
	}

	// Get user comments from service
	response, err := h.userService.GetUserComments(r.Context(), userID, cursorPtr, limit)
	if err != nil {
		if err.Error() == "user not found" {
			writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "GET_USER_COMMENTS_FAILED", "Failed to get user comments")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// UpdateMe handles PATCH /api/v1/users/me
func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PATCH requests are allowed")
		return
	}

	// Get user ID from context (set by auth middleware)
	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Parse request body
	var req models.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Update profile
	response, err := h.userService.UpdateProfile(r.Context(), userID, &req)
	if err != nil {
		switch err.Error() {
		case "user not found":
			writeError(w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
		case "at least one field (bio or profile_picture_url) is required":
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		case "invalid profile picture URL":
			writeError(w, http.StatusBadRequest, "INVALID_URL", err.Error())
		case "profile picture URL must use http or https scheme":
			writeError(w, http.StatusBadRequest, "INVALID_URL_SCHEME", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "UPDATE_FAILED", "Failed to update profile")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// GetMySectionSubscriptions handles GET /api/v1/users/me/section-subscriptions
func (h *UserHandler) GetMySectionSubscriptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	subscriptions, err := h.userService.GetSectionSubscriptions(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GET_SECTION_SUBSCRIPTIONS_FAILED", "Failed to get section subscriptions")
		return
	}

	response := models.GetSectionSubscriptionsResponse{
		SectionSubscriptions: subscriptions,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// UpdateMySectionSubscription handles PATCH /api/v1/users/me/section-subscriptions/{sectionId}
func (h *UserHandler) UpdateMySectionSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PATCH requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 7 || pathParts[5] != "section-subscriptions" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Section ID is required")
		return
	}

	sectionIDStr := pathParts[6]
	sectionID, err := uuid.Parse(sectionIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_SECTION_ID", "Invalid section ID format")
		return
	}

	var req models.UpdateSectionSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	if req.OptedOut == nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "opted_out is required")
		return
	}

	response, err := h.userService.UpdateSectionSubscription(r.Context(), userID, sectionID, *req.OptedOut)
	if err != nil {
		switch err.Error() {
		case "section not found":
			writeError(w, http.StatusNotFound, "SECTION_NOT_FOUND", "Section not found")
		default:
			writeError(w, http.StatusInternalServerError, "UPDATE_SECTION_SUBSCRIPTION_FAILED", "Failed to update section subscription")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
