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

// AdminHandler handles admin-specific endpoints
type AdminHandler struct {
	userService    *services.UserService
	postService    *services.PostService
	commentService *services.CommentService
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(db *sql.DB) *AdminHandler {
	return &AdminHandler{
		userService:    services.NewUserService(db),
		postService:    services.NewPostService(db),
		commentService: services.NewCommentService(db),
	}
}

// ListPendingUsers returns all users pending admin approval
func (h *AdminHandler) ListPendingUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	pendingUsers, err := h.userService.GetPendingUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_FAILED", "Failed to fetch pending users")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(pendingUsers)
}

// ApproveUser approves a pending user (sets approved_at timestamp)
func (h *AdminHandler) ApproveUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PATCH requests are allowed")
		return
	}

	// Extract user ID from URL path: /admin/users/{id}/approve
	userIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/")
	userIDStr = strings.TrimSuffix(userIDStr, "/approve")

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID format")
		return
	}

	approveResponse, err := h.userService.ApproveUser(r.Context(), userID)
	if err != nil {
		// Determine appropriate error code and status
		switch err.Error() {
		case "user not found":
			writeError(w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
		case "user already approved":
			writeError(w, http.StatusConflict, "USER_ALREADY_APPROVED", err.Error())
		case "user has been deleted":
			writeError(w, http.StatusGone, "USER_DELETED", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "APPROVAL_FAILED", "Failed to approve user")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(approveResponse)
}

// RejectUser rejects a pending user (hard delete)
func (h *AdminHandler) RejectUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	// Extract user ID from URL path: /admin/users/{id}
	userIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/")

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID format")
		return
	}

	rejectResponse, err := h.userService.RejectUser(r.Context(), userID)
	if err != nil {
		// Determine appropriate error code and status
		switch err.Error() {
		case "user not found":
			writeError(w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
		case "cannot reject approved user":
			writeError(w, http.StatusConflict, "USER_ALREADY_APPROVED", "Cannot reject an already approved user")
		default:
			writeError(w, http.StatusInternalServerError, "REJECTION_FAILED", "Failed to reject user")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rejectResponse)
}

// HardDeletePost permanently deletes a post (admin only)
func (h *AdminHandler) HardDeletePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	// Extract admin user ID from context
	adminUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Extract post ID from URL path: /admin/posts/{id}
	postIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/posts/")

	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	err = h.postService.HardDeletePost(r.Context(), postID, adminUserID)
	if err != nil {
		switch err.Error() {
		case "post not found":
			writeError(w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "DELETE_FAILED", "Failed to delete post")
		}
		return
	}

	response := models.HardDeletePostResponse{
		ID:      postID,
		Message: "Post permanently deleted",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// HardDeleteComment permanently deletes a comment (admin only)
func (h *AdminHandler) HardDeleteComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	// Extract admin user ID from context
	adminUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Extract comment ID from URL path: /admin/comments/{id}
	commentIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/comments/")

	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_COMMENT_ID", "Invalid comment ID format")
		return
	}

	err = h.commentService.HardDeleteComment(r.Context(), commentID, adminUserID)
	if err != nil {
		switch err.Error() {
		case "comment not found":
			writeError(w, http.StatusNotFound, "COMMENT_NOT_FOUND", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "DELETE_FAILED", "Failed to delete comment")
		}
		return
	}

	response := models.HardDeleteCommentResponse{
		ID:      commentID,
		Message: "Comment permanently deleted",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
