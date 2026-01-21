package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
)

// CommentHandler handles comment endpoints
type CommentHandler struct {
	commentService *services.CommentService
	userService    *services.UserService
	postService    *services.PostService
	notify         *services.NotificationService
	redis          *redis.Client
}

// NewCommentHandler creates a new comment handler
func NewCommentHandler(db *sql.DB, redisClient *redis.Client) *CommentHandler {
	return &CommentHandler{
		commentService: services.NewCommentService(db),
		userService:    services.NewUserService(db),
		postService:    services.NewPostService(db),
		notify:         services.NewNotificationService(db),
		redis:          redisClient,
	}
}

// CreateComment handles POST /api/v1/comments
func (h *CommentHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	// Parse request body
	var req models.CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Create comment
	comment, err := h.commentService.CreateComment(r.Context(), &req, userID)
	if err != nil {
		// Determine appropriate error code and status
		switch err.Error() {
		case "post_id is required":
			writeError(r.Context(), w, http.StatusBadRequest, "POST_ID_REQUIRED", err.Error())
		case "invalid post id":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_POST_ID", err.Error())
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		case "invalid parent comment id":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_PARENT_COMMENT_ID", err.Error())
		case "parent comment not found":
			writeError(r.Context(), w, http.StatusNotFound, "PARENT_COMMENT_NOT_FOUND", err.Error())
		case "content is required":
			writeError(r.Context(), w, http.StatusBadRequest, "CONTENT_REQUIRED", err.Error())
		case "content must be less than 5000 characters":
			writeError(r.Context(), w, http.StatusBadRequest, "CONTENT_TOO_LONG", err.Error())
		case "link url cannot be empty":
			writeError(r.Context(), w, http.StatusBadRequest, "LINK_URL_REQUIRED", err.Error())
		case "link url must be less than 2048 characters":
			writeError(r.Context(), w, http.StatusBadRequest, "LINK_URL_TOO_LONG", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "COMMENT_CREATION_FAILED", "Failed to create comment")
		}
		return
	}

	response := models.CreateCommentResponse{
		Comment: *comment,
	}

	publishCtx, cancel := publishContext()
	_ = h.notify.CreateNotificationForPostComment(publishCtx, comment.PostID, comment.ID, userID)
	mentionedUserIDs, _ := resolveMentionedUserIDs(publishCtx, h.userService, comment.Content, userID)
	_ = publishEvent(publishCtx, h.redis, formatChannel(postPrefix, comment.PostID), "new_comment", commentEventData{Comment: comment})
	if sectionID, err := h.postService.GetSectionIDByPostID(publishCtx, comment.PostID); err == nil {
		_ = h.notify.CreateMentionNotifications(publishCtx, mentionedUserIDs, userID, sectionID, comment.PostID, &comment.ID)
		_ = publishEvent(publishCtx, h.redis, formatChannel(sectionPrefix, sectionID), "new_comment", commentEventData{Comment: comment})
	}
	_ = publishMentions(publishCtx, h.redis, mentionedUserIDs, userID, &comment.PostID, &comment.ID)
	cancel()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetComment handles GET /api/v1/comments/{id}
func (h *CommentHandler) GetComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	// Extract comment ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Comment ID is required")
		return
	}

	commentIDStr := pathParts[4]
	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_COMMENT_ID", "Invalid comment ID format")
		return
	}

	// Get comment from service
	userID, _ := middleware.GetUserIDFromContext(r.Context())
	comment, err := h.commentService.GetCommentByID(r.Context(), commentID, userID)
	if err != nil {
		if err.Error() == "comment not found" {
			writeError(r.Context(), w, http.StatusNotFound, "COMMENT_NOT_FOUND", "Comment not found")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_COMMENT_FAILED", "Failed to get comment")
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

// GetThread handles GET /api/v1/posts/{postId}/comments
func (h *CommentHandler) GetThread(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	// Extract post ID from URL path: /api/v1/posts/{postId}/comments
	pathParts := strings.Split(r.URL.Path, "/")
	// pathParts: ["", "api", "v1", "posts", "{postId}", "comments"]
	if len(pathParts) < 6 {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Post ID is required")
		return
	}

	postIDStr := pathParts[4]
	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	// Parse query parameters
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	cursor := r.URL.Query().Get("cursor")
	var cursorPtr *string
	if cursor != "" {
		cursorPtr = &cursor
	}

	// Get thread comments
	userID, _ := middleware.GetUserIDFromContext(r.Context())
	comments, nextCursor, hasMore, err := h.commentService.GetThreadComments(r.Context(), postID, limit, cursorPtr, userID)
	if err != nil {
		if err.Error() == "post not found" {
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
			return
		}
		if err.Error() == "invalid cursor" {
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_CURSOR", "Invalid cursor format")
			return
		}
		if err.Error() == "cursor not found" {
			writeError(r.Context(), w, http.StatusBadRequest, "CURSOR_NOT_FOUND", "Cursor not found")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_THREAD_FAILED", "Failed to get thread")
		return
	}

	// Return response
	response := models.GetThreadResponse{
		Comments: comments,
		Meta: models.PageMeta{
			Cursor:  nextCursor,
			HasMore: hasMore,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// DeleteComment handles DELETE /api/v1/comments/{id}
func (h *CommentHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	isAdmin, err := middleware.GetIsAdminFromContext(r.Context())
	if err != nil {
		isAdmin = false
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Comment ID is required")
		return
	}

	commentIDStr := pathParts[4]
	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_COMMENT_ID", "Invalid comment ID format")
		return
	}

	comment, err := h.commentService.DeleteComment(r.Context(), commentID, userID, isAdmin)
	if err != nil {
		switch err.Error() {
		case "comment not found":
			writeError(r.Context(), w, http.StatusNotFound, "COMMENT_NOT_FOUND", "Comment not found")
		case "unauthorized to delete this comment":
			writeError(r.Context(), w, http.StatusForbidden, "FORBIDDEN", "You can only delete your own comments")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "COMMENT_DELETION_FAILED", "Failed to delete comment")
		}
		return
	}

	response := models.DeleteCommentResponse{
		Comment: comment,
		Message: "Comment deleted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// RestoreComment handles POST /api/v1/comments/{id}/restore
func (h *CommentHandler) RestoreComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	session, err := middleware.GetUserFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user session")
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Comment ID is required")
		return
	}

	commentIDStr := pathParts[4]
	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_COMMENT_ID", "Invalid comment ID format")
		return
	}

	comment, err := h.commentService.RestoreComment(r.Context(), commentID, userID, session.IsAdmin)
	if err != nil {
		switch err.Error() {
		case "comment not found":
			writeError(r.Context(), w, http.StatusNotFound, "COMMENT_NOT_FOUND", "Comment not found")
		case "unauthorized":
			writeError(r.Context(), w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to restore this comment")
		case "comment permanently deleted":
			writeError(r.Context(), w, http.StatusGone, "COMMENT_PERMANENTLY_DELETED", "Comment was permanently deleted more than 7 days ago")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "RESTORE_FAILED", "Failed to restore comment")
		}
		return
	}

	response := models.RestoreCommentResponse{
		Comment: *comment,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
