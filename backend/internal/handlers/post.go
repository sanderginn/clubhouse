package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

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

// CreatePost handles POST /posts
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
