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

// PostHandler handles post endpoints
type PostHandler struct {
	postService *services.PostService
	userService *services.UserService
	notify      *services.NotificationService
	redis       *redis.Client
	rateLimiter contentRateLimiter
}

// NewPostHandler creates a new post handler
func NewPostHandler(db *sql.DB, redisClient *redis.Client, pushService *services.PushService) *PostHandler {
	return &PostHandler{
		postService: services.NewPostService(db),
		userService: services.NewUserService(db),
		notify:      services.NewNotificationService(db, pushService),
		redis:       redisClient,
		rateLimiter: services.NewPostRateLimiter(redisClient),
	}
}

// CreatePost handles POST /api/v1/posts
func (h *PostHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
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
	var req models.CreatePostRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Create post
	post, err := h.postService.CreatePost(r.Context(), &req, userID)
	if err != nil {
		// Determine appropriate error code and status
		switch err.Error() {
		case "section_id is required":
			writeError(r.Context(), w, http.StatusBadRequest, "SECTION_ID_REQUIRED", err.Error())
		case "invalid section id":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_SECTION_ID", err.Error())
		case "section not found":
			writeError(r.Context(), w, http.StatusNotFound, "SECTION_NOT_FOUND", err.Error())
		case "content is required":
			writeError(r.Context(), w, http.StatusBadRequest, "CONTENT_REQUIRED", err.Error())
		case "content must be less than 5000 characters":
			writeError(r.Context(), w, http.StatusBadRequest, "CONTENT_TOO_LONG", err.Error())
		case "link url cannot be empty":
			writeError(r.Context(), w, http.StatusBadRequest, "LINK_URL_REQUIRED", err.Error())
		case "link url must be less than 2048 characters":
			writeError(r.Context(), w, http.StatusBadRequest, "LINK_URL_TOO_LONG", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "POST_CREATION_FAILED", "Failed to create post")
		}
		return
	}

	if user, err := h.userService.GetUserByID(r.Context(), userID); err == nil {
		post.User = user
	}

	response := models.CreatePostResponse{
		Post: *post,
	}

	publishCtx, cancel := publishContext()
	_ = h.notify.CreateNotificationsForNewPost(publishCtx, post.ID, post.SectionID, userID)
	mentionedUserIDs, _ := resolveMentionedUserIDs(publishCtx, h.userService, post.Content, userID)
	_ = h.notify.CreateMentionNotifications(publishCtx, mentionedUserIDs, userID, post.SectionID, post.ID, nil)
	_ = publishEvent(publishCtx, h.redis, formatChannel(sectionPrefix, post.SectionID), "new_post", postEventData{Post: post})
	_ = publishMentions(publishCtx, h.redis, mentionedUserIDs, userID, &post.ID, nil)
	cancel()

	observability.LogInfo(r.Context(), "post created",
		"post_id", post.ID.String(),
		"user_id", userID.String(),
		"section_id", post.SectionID.String(),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode create post response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusCreated,
			Err:        err,
		})
	}
}

// UpdatePost handles PATCH /api/v1/posts/{id}
func (h *PostHandler) UpdatePost(w http.ResponseWriter, r *http.Request) {
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
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Post ID is required")
		return
	}

	postIDStr := pathParts[4]
	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	var req models.UpdatePostRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	post, err := h.postService.UpdatePost(r.Context(), postID, userID, &req)
	if err != nil {
		switch err.Error() {
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
		case "unauthorized to edit this post":
			writeError(r.Context(), w, http.StatusForbidden, "FORBIDDEN", "You can only edit your own posts")
		case "content is required":
			writeError(r.Context(), w, http.StatusBadRequest, "CONTENT_REQUIRED", err.Error())
		case "content must be less than 5000 characters":
			writeError(r.Context(), w, http.StatusBadRequest, "CONTENT_TOO_LONG", err.Error())
		case "link url cannot be empty":
			writeError(r.Context(), w, http.StatusBadRequest, "LINK_URL_REQUIRED", err.Error())
		case "link url must be less than 2048 characters":
			writeError(r.Context(), w, http.StatusBadRequest, "LINK_URL_TOO_LONG", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "POST_UPDATE_FAILED", "Failed to update post")
		}
		return
	}

	response := models.UpdatePostResponse{
		Post: *post,
	}

	publishCtx, cancel := publishContext()
	mentionedUserIDs, _ := resolveMentionedUserIDs(publishCtx, h.userService, post.Content, userID)
	_ = h.notify.CreateMentionNotifications(publishCtx, mentionedUserIDs, userID, post.SectionID, post.ID, nil)
	_ = publishMentions(publishCtx, h.redis, mentionedUserIDs, userID, &post.ID, nil)
	cancel()

	observability.LogInfo(r.Context(), "post updated",
		"post_id", post.ID.String(),
		"user_id", userID.String(),
		"section_id", post.SectionID.String(),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode update post response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// GetPost handles GET /api/v1/posts/{id}
func (h *PostHandler) GetPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	// Extract post ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Post ID is required")
		return
	}

	postIDStr := pathParts[4]
	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	// Get post from service
	userID, _ := middleware.GetUserIDFromContext(r.Context())
	post, err := h.postService.GetPostByID(r.Context(), postID, userID)
	if err != nil {
		if err.Error() == "post not found" {
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_POST_FAILED", "Failed to get post")
		return
	}

	// Return post response
	response := models.GetPostResponse{
		Post: post,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode get post response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// GetFeed handles GET /api/v1/sections/{sectionId}/feed
func (h *PostHandler) GetFeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	// Extract section ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Section ID is required")
		return
	}

	sectionIDStr := pathParts[4]
	sectionID, err := uuid.Parse(sectionIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_SECTION_ID", "Invalid section ID format")
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

	userID, _ := middleware.GetUserIDFromContext(r.Context())
	feed, err := h.postService.GetFeed(r.Context(), sectionID, cursorPtr, limit, userID)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_FEED_FAILED", "Failed to get feed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(feed); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode feed response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// DeletePost handles DELETE /api/v1/posts/{id}
func (h *PostHandler) DeletePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
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
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Post ID is required")
		return
	}

	postIDStr := pathParts[4]
	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	// Delete post
	post, err := h.postService.DeletePost(r.Context(), postID, userID, isAdmin)
	if err != nil {
		// Determine appropriate error code and status
		switch err.Error() {
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
		case "unauthorized to delete this post":
			writeError(r.Context(), w, http.StatusForbidden, "UNAUTHORIZED", "You can only delete your own posts")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "POST_DELETION_FAILED", "Failed to delete post")
		}
		return
	}

	response := models.DeletePostResponse{
		Post:    post,
		Message: "Post deleted successfully",
	}

	observability.LogInfo(r.Context(), "post deleted",
		"post_id", post.ID.String(),
		"user_id", userID.String(),
		"section_id", post.SectionID.String(),
		"is_admin", strconv.FormatBool(isAdmin),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode delete post response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// parseIntParam parses a string parameter as an integer
func parseIntParam(s string) (int, error) {
	return strconv.Atoi(s)
}

// RestorePost handles POST /api/v1/posts/{id}/restore
func (h *PostHandler) RestorePost(w http.ResponseWriter, r *http.Request) {
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

	// Get user session to check if admin
	session, err := middleware.GetUserFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user session")
		return
	}

	// Extract post ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Post ID is required")
		return
	}

	postIDStr := pathParts[4]
	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	// Restore post
	post, err := h.postService.RestorePost(r.Context(), postID, userID, session.IsAdmin)
	if err != nil {
		// Determine appropriate error code and status
		switch err.Error() {
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "Post not found")
		case "unauthorized":
			writeError(r.Context(), w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to restore this post")
		case "post permanently deleted":
			writeError(r.Context(), w, http.StatusGone, "POST_PERMANENTLY_DELETED", "Post was permanently deleted more than 7 days ago")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "RESTORE_FAILED", "Failed to restore post")
		}
		return
	}

	response := models.RestorePostResponse{
		Post: *post,
	}

	observability.LogInfo(r.Context(), "post restored",
		"post_id", post.ID.String(),
		"user_id", userID.String(),
		"section_id", post.SectionID.String(),
		"is_admin", strconv.FormatBool(session.IsAdmin),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode restore post response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}
