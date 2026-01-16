package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
)

// CommentHandler handles comment endpoints
type CommentHandler struct {
	commentService *services.CommentService
}

// NewCommentHandler creates a new comment handler
func NewCommentHandler(db *sql.DB) *CommentHandler {
	return &CommentHandler{
		commentService: services.NewCommentService(db),
	}
}

// CreateComment handles POST /api/v1/comments
func (h *CommentHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
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
	var req models.CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Create comment
	comment, err := h.commentService.CreateComment(r.Context(), &req, userID)
	if err != nil {
		// Determine appropriate error code and status
		switch err.Error() {
		case "post_id is required":
			writeError(w, http.StatusBadRequest, "POST_ID_REQUIRED", err.Error())
		case "invalid post id":
			writeError(w, http.StatusBadRequest, "INVALID_POST_ID", err.Error())
		case "post not found":
			writeError(w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		case "invalid parent comment id":
			writeError(w, http.StatusBadRequest, "INVALID_PARENT_COMMENT_ID", err.Error())
		case "parent comment not found":
			writeError(w, http.StatusNotFound, "PARENT_COMMENT_NOT_FOUND", err.Error())
		case "content is required":
			writeError(w, http.StatusBadRequest, "CONTENT_REQUIRED", err.Error())
		case "content must be less than 5000 characters":
			writeError(w, http.StatusBadRequest, "CONTENT_TOO_LONG", err.Error())
		case "link url cannot be empty":
			writeError(w, http.StatusBadRequest, "LINK_URL_REQUIRED", err.Error())
		case "link url must be less than 2048 characters":
			writeError(w, http.StatusBadRequest, "LINK_URL_TOO_LONG", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "COMMENT_CREATION_FAILED", "Failed to create comment")
		}
		return
	}

	response := models.CreateCommentResponse{
		Comment: *comment,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetComment handles GET /api/v1/comments/{id}
func (h *CommentHandler) GetComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	// Extract comment ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Comment ID is required")
		return
	}

	commentIDStr := pathParts[4]
	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_COMMENT_ID", "Invalid comment ID format")
		return
	}

	// Get comment from service
	comment, err := h.commentService.GetCommentByID(r.Context(), commentID)
	if err != nil {
		if err.Error() == "comment not found" {
			writeError(w, http.StatusNotFound, "COMMENT_NOT_FOUND", "Comment not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "GET_COMMENT_FAILED", "Failed to get comment")
		return
	}

	// Return comment response
	response := models.GetCommentResponse{
		Comment: comment,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
