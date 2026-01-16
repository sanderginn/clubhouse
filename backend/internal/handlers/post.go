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

// PostHandler handles post endpoints
type PostHandler struct {
	postService *services.PostService
}

// NewPostHandler creates a new post handler
func NewPostHandler(db *sql.DB) *PostHandler {
	return &PostHandler{
		postService: services.NewPostService(db),
	}
}

// CreatePost handles POST /api/v1/posts
func (h *PostHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	// Parse request body
	var req models.CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Create post
	post, err := h.postService.CreatePost(r.Context(), &req, userID)
	if err != nil {
		// Determine appropriate error code and status
		switch err.Error() {
		case "section_id is required":
			writeError(w, http.StatusBadRequest, "SECTION_ID_REQUIRED", err.Error())
		case "invalid section id":
			writeError(w, http.StatusBadRequest, "INVALID_SECTION_ID", err.Error())
		case "section not found":
			writeError(w, http.StatusNotFound, "SECTION_NOT_FOUND", err.Error())
		case "content is required":
			writeError(w, http.StatusBadRequest, "CONTENT_REQUIRED", err.Error())
		case "content must be less than 5000 characters":
			writeError(w, http.StatusBadRequest, "CONTENT_TOO_LONG", err.Error())
		case "link url cannot be empty":
			writeError(w, http.StatusBadRequest, "LINK_URL_REQUIRED", err.Error())
		case "link url must be less than 2048 characters":
			writeError(w, http.StatusBadRequest, "LINK_URL_TOO_LONG", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "POST_CREATION_FAILED", "Failed to create post")
		}
		return
	}

	response := models.CreatePostResponse{
		Post: *post,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetPost handles GET /api/v1/posts/{id}
func (h *PostHandler) GetPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	// Extract post ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Post ID is required")
		return
	}

	postIDStr := pathParts[4]
	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	// Get post from service
	post, err := h.postService.GetPostByID(r.Context(), postID)
	if err != nil {
		if err.Error() == "post not found" {
			writeError(w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "GET_POST_FAILED", "Failed to get post")
		return
	}

	// Return post response
	response := models.GetPostResponse{
		Post: post,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// GetFeed handles GET /api/v1/sections/{sectionId}/feed
func (h *PostHandler) GetFeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	// Extract section ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Section ID is required")
		return
	}

	sectionIDStr := pathParts[4]
	sectionID, err := uuid.Parse(sectionIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_SECTION_ID", "Invalid section ID format")
		return
	}

	// Parse query parameters
	cursor := r.URL.Query().Get("cursor")
	limitStr := r.URL.Query().Get("limit")

	limit := 20
	if limitStr != "" {
		if parsedLimit, err := parseIntParam(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Clamp limit to reasonable range
	if limit > 100 {
		limit = 100
	}

	// Get feed from service
	var cursorPtr *string
	if cursor != "" {
		cursorPtr = &cursor
	}

	feed, err := h.postService.GetFeed(r.Context(), sectionID, cursorPtr, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GET_FEED_FAILED", "Failed to get feed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(feed)
}

// DeletePost handles DELETE /api/v1/posts/{id}
func (h *PostHandler) DeletePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	// Get isAdmin flag from context
	isAdmin, err := middleware.GetIsAdminFromContext(r.Context())
	if err != nil {
		// If admin flag is not set, default to false
		isAdmin = false
	}

	// Extract post ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Post ID is required")
		return
	}

	postIDStr := pathParts[4]
	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	// Delete post
	post, err := h.postService.DeletePost(r.Context(), postID, userID, isAdmin)
	if err != nil {
		// Determine appropriate error code and status
		switch err.Error() {
		case "post not found":
			writeError(w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
		case "unauthorized to delete this post":
			writeError(w, http.StatusForbidden, "UNAUTHORIZED", "You can only delete your own posts")
		default:
			writeError(w, http.StatusInternalServerError, "POST_DELETION_FAILED", "Failed to delete post")
		}
		return
	}

	response := models.DeletePostResponse{
		Post:    post,
		Message: "Post deleted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// parseIntParam parses a string parameter as an integer
func parseIntParam(s string) (int, error) {
	return strconv.Atoi(s)
}
