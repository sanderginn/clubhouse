package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

// CookLogHandler handles cook log endpoints.
type CookLogHandler struct {
	cookLogService *services.CookLogService
	postService    *services.PostService
	userService    *services.UserService
	redis          *redis.Client
}

// NewCookLogHandler creates a new cook log handler.
func NewCookLogHandler(db *sql.DB, redisClient *redis.Client) *CookLogHandler {
	return &CookLogHandler{
		cookLogService: services.NewCookLogService(db),
		postService:    services.NewPostService(db),
		userService:    services.NewUserService(db),
		redis:          redisClient,
	}
}

// LogCook handles POST /api/v1/posts/{postId}/cook-log.
func (h *CookLogHandler) LogCook(w http.ResponseWriter, r *http.Request) {
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

	var req models.CreateCookLogRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	cookLog, err := h.cookLogService.LogCook(r.Context(), userID, postID, req.Rating, req.Notes)
	if err != nil {
		switch err.Error() {
		case "rating must be between 1 and 5":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_RATING", err.Error())
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		case "post is not a recipe":
			writeError(r.Context(), w, http.StatusBadRequest, "POST_NOT_RECIPE", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "COOK_LOG_CREATE_FAILED", "Failed to log cook")
		}
		return
	}

	publishCtx, cancel := publishContext()
	username := ""
	if user, err := h.userService.GetUserByID(publishCtx, userID); err == nil {
		username = user.Username
	} else {
		observability.LogWarn(publishCtx, "failed to load user for recipe_cooked event",
			"user_id", userID.String(),
			"post_id", postID.String(),
			"error", err.Error(),
		)
	}
	eventData := recipeCookedEventData{
		PostID:   postID,
		UserID:   userID,
		Username: username,
		Rating:   cookLog.Rating,
	}
	_ = publishEvent(publishCtx, h.redis, formatChannel(postPrefix, postID), "recipe_cooked", eventData)
	if sectionID, err := h.postService.GetSectionIDByPostID(publishCtx, postID); err == nil {
		_ = publishEvent(publishCtx, h.redis, formatChannel(sectionPrefix, sectionID), "recipe_cooked", eventData)
	}
	cancel()

	observability.RecordCookLogCreated(r.Context())
	observability.LogInfo(r.Context(), "cook log created",
		"cook_log_id", cookLog.ID.String(),
		"user_id", userID.String(),
		"post_id", postID.String(),
		"rating", strconv.Itoa(cookLog.Rating),
	)

	response := models.CreateCookLogResponse{
		CookLog: *cookLog,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode create cook log response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusCreated,
			Err:        err,
		})
	}
}

// UpdateCookLog handles PUT /api/v1/posts/{postId}/cook-log.
func (h *CookLogHandler) UpdateCookLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PUT requests are allowed")
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

	var req models.UpdateCookLogRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if req.Rating == nil {
		writeError(r.Context(), w, http.StatusBadRequest, "RATING_REQUIRED", "Rating is required")
		return
	}

	cookLog, err := h.cookLogService.UpdateCookLog(r.Context(), userID, postID, *req.Rating, req.Notes)
	if err != nil {
		switch err.Error() {
		case "rating must be between 1 and 5":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_RATING", err.Error())
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		case "post is not a recipe":
			writeError(r.Context(), w, http.StatusBadRequest, "POST_NOT_RECIPE", err.Error())
		case "cook log not found":
			writeError(r.Context(), w, http.StatusNotFound, "COOK_LOG_NOT_FOUND", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "COOK_LOG_UPDATE_FAILED", "Failed to update cook log")
		}
		return
	}

	observability.RecordCookLogUpdated(r.Context())
	observability.LogInfo(r.Context(), "cook log updated",
		"cook_log_id", cookLog.ID.String(),
		"user_id", userID.String(),
		"post_id", postID.String(),
		"rating", strconv.Itoa(cookLog.Rating),
	)

	response := models.UpdateCookLogResponse{
		CookLog: *cookLog,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode update cook log response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// RemoveCookLog handles DELETE /api/v1/posts/{postId}/cook-log.
func (h *CookLogHandler) RemoveCookLog(w http.ResponseWriter, r *http.Request) {
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

	if err := h.cookLogService.RemoveCookLog(r.Context(), userID, postID); err != nil {
		switch err.Error() {
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		case "post is not a recipe":
			writeError(r.Context(), w, http.StatusBadRequest, "POST_NOT_RECIPE", err.Error())
		case "cook log not found":
			writeError(r.Context(), w, http.StatusNotFound, "COOK_LOG_NOT_FOUND", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "COOK_LOG_DELETE_FAILED", "Failed to remove cook log")
		}
		return
	}

	publishCtx, cancel := publishContext()
	eventData := recipeCookRemovedEventData{
		PostID: postID,
		UserID: userID,
	}
	_ = publishEvent(publishCtx, h.redis, formatChannel(postPrefix, postID), "recipe_cook_removed", eventData)
	if sectionID, err := h.postService.GetSectionIDByPostID(publishCtx, postID); err == nil {
		_ = publishEvent(publishCtx, h.redis, formatChannel(sectionPrefix, sectionID), "recipe_cook_removed", eventData)
	}
	cancel()

	observability.RecordCookLogRemoved(r.Context())
	observability.LogInfo(r.Context(), "cook log removed",
		"user_id", userID.String(),
		"post_id", postID.String(),
	)

	w.WriteHeader(http.StatusNoContent)
}

// GetPostCookLogs handles GET /api/v1/posts/{postId}/cook-logs.
func (h *CookLogHandler) GetPostCookLogs(w http.ResponseWriter, r *http.Request) {
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

	info, err := h.cookLogService.GetPostCookLogs(r.Context(), postID, &userID)
	if err != nil {
		switch err.Error() {
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		case "post is not a recipe":
			writeError(r.Context(), w, http.StatusBadRequest, "POST_NOT_RECIPE", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "GET_COOK_LOGS_FAILED", "Failed to get cook logs")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode get post cook logs response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// GetMyCookLogs handles GET /api/v1/me/cook-logs.
func (h *CookLogHandler) GetMyCookLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100
	}

	cursor := r.URL.Query().Get("cursor")
	var cursorPtr *string
	if cursor != "" {
		cursorPtr = &cursor
	}

	logs, hasMore, nextCursor, err := h.cookLogService.GetUserCookLogs(r.Context(), userID, limit, cursorPtr)
	if err != nil {
		switch err.Error() {
		case "user not found":
			writeError(r.Context(), w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "GET_COOK_LOGS_FAILED", "Failed to get cook logs")
		}
		return
	}

	response := models.ListCookLogsResponse{
		CookLogs: logs,
		Meta: models.PageMeta{
			Cursor:  nextCursor,
			HasMore: hasMore,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode list cook logs response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}
