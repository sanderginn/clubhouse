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
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

// CommentHandler handles comment endpoints
type CommentHandler struct {
	commentService *services.CommentService
	userService    *services.UserService
	postService    *services.PostService
	notify         *services.NotificationService
	redis          *redis.Client
	rateLimiter    contentRateLimiter
}

// NewCommentHandler creates a new comment handler
func NewCommentHandler(db *sql.DB, redisClient *redis.Client, pushService *services.PushService) *CommentHandler {
	return &CommentHandler{
		commentService: services.NewCommentService(db),
		userService:    services.NewUserService(db),
		postService:    services.NewPostService(db),
		notify:         services.NewNotificationService(db, pushService),
		redis:          redisClient,
		rateLimiter:    services.NewCommentRateLimiter(redisClient),
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

	if !checkContentRateLimit(r.Context(), w, h.rateLimiter, userID.String()) {
		return
	}

	// Parse request body
	var req models.CreateCommentRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
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
		case "invalid image id":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_IMAGE_ID", err.Error())
		case "image not found":
			writeError(r.Context(), w, http.StatusNotFound, "IMAGE_NOT_FOUND", err.Error())
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

	mentioningUser := userSummaryFromUser(comment.User)
	contentExcerpt := truncateMentionExcerpt(comment.Content)

	publishCtx, cancel := publishContext()
	_ = h.notify.CreateNotificationForPostComment(publishCtx, comment.PostID, comment.ID, userID)
	mentionedUserIDs, _ := resolveMentionedUserIDs(publishCtx, h.userService, comment.Content, userID)
	_ = publishEvent(publishCtx, h.redis, formatChannel(postPrefix, comment.PostID), "new_comment", commentEventData{Comment: comment})
	if sectionID, err := h.postService.GetSectionIDByPostID(publishCtx, comment.PostID); err == nil {
		_ = h.notify.CreateMentionNotifications(publishCtx, mentionedUserIDs, userID, sectionID, comment.PostID, &comment.ID)
		_ = publishEvent(publishCtx, h.redis, formatChannel(sectionPrefix, sectionID), "new_comment", commentEventData{Comment: comment})
	}
	_ = publishMentions(publishCtx, h.redis, mentionedUserIDs, userID, &comment.PostID, &comment.ID, mentioningUser, contentExcerpt)
	cancel()

	sectionID := ""
	if comment.SectionID != nil {
		sectionID = comment.SectionID.String()
	}
	observability.LogInfo(r.Context(), "comment created",
		"comment_id", comment.ID.String(),
		"user_id", userID.String(),
		"post_id", comment.PostID.String(),
		"section_id", sectionID,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode create comment response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusCreated,
			Err:        err,
		})
	}
}

// UpdateComment handles PATCH /api/v1/comments/{id}
func (h *CommentHandler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PATCH requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
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

	var req models.UpdateCommentRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	comment, err := h.commentService.UpdateComment(r.Context(), commentID, userID, &req)
	if err != nil {
		switch err.Error() {
		case "comment not found":
			writeError(r.Context(), w, http.StatusNotFound, "COMMENT_NOT_FOUND", "Comment not found")
		case "unauthorized to edit this comment":
			writeError(r.Context(), w, http.StatusForbidden, "FORBIDDEN", "You can only edit your own comments")
		case "content is required":
			writeError(r.Context(), w, http.StatusBadRequest, "CONTENT_REQUIRED", err.Error())
		case "content must be less than 5000 characters":
			writeError(r.Context(), w, http.StatusBadRequest, "CONTENT_TOO_LONG", err.Error())
		case "link url cannot be empty":
			writeError(r.Context(), w, http.StatusBadRequest, "LINK_URL_REQUIRED", err.Error())
		case "link url must be less than 2048 characters":
			writeError(r.Context(), w, http.StatusBadRequest, "LINK_URL_TOO_LONG", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "COMMENT_UPDATE_FAILED", "Failed to update comment")
		}
		return
	}
	observability.RecordCommentUpdated(r.Context())

	response := models.UpdateCommentResponse{
		Comment: *comment,
	}

	publishCtx, cancel := publishContext()
	mentionedUserIDs, _ := resolveMentionedUserIDs(publishCtx, h.userService, comment.Content, userID)
	if comment.SectionID != nil {
		_ = h.notify.CreateMentionNotifications(publishCtx, mentionedUserIDs, userID, *comment.SectionID, comment.PostID, &comment.ID)
	} else if sectionID, err := h.postService.GetSectionIDByPostID(publishCtx, comment.PostID); err == nil {
		_ = h.notify.CreateMentionNotifications(publishCtx, mentionedUserIDs, userID, sectionID, comment.PostID, &comment.ID)
	}
	mentioningUser := userSummaryFromUser(comment.User)
	if mentioningUser == nil {
		if user, err := h.userService.GetUserByID(publishCtx, userID); err == nil {
			mentioningUser = userSummaryFromUser(user)
		}
	}
	contentExcerpt := truncateMentionExcerpt(comment.Content)
	_ = publishMentions(publishCtx, h.redis, mentionedUserIDs, userID, &comment.PostID, &comment.ID, mentioningUser, contentExcerpt)
	cancel()

	sectionID := ""
	if comment.SectionID != nil {
		sectionID = comment.SectionID.String()
	}
	observability.LogInfo(r.Context(), "comment updated",
		"comment_id", comment.ID.String(),
		"user_id", userID.String(),
		"post_id", comment.PostID.String(),
		"section_id", sectionID,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode update comment response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
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
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode get comment response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
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
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode thread response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
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

	sectionID := ""
	if comment.SectionID != nil {
		sectionID = comment.SectionID.String()
	}
	observability.LogInfo(r.Context(), "comment deleted",
		"comment_id", comment.ID.String(),
		"user_id", userID.String(),
		"post_id", comment.PostID.String(),
		"section_id", sectionID,
		"is_admin", strconv.FormatBool(isAdmin),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode delete comment response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
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

	sectionID := ""
	if comment.SectionID != nil {
		sectionID = comment.SectionID.String()
	}
	observability.LogInfo(r.Context(), "comment restored",
		"comment_id", comment.ID.String(),
		"user_id", userID.String(),
		"post_id", comment.PostID.String(),
		"section_id", sectionID,
		"is_admin", strconv.FormatBool(session.IsAdmin),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode restore comment response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}
