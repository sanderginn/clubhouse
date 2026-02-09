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

// WatchLogHandler handles movie/series watch log endpoints.
type WatchLogHandler struct {
	watchLogService *services.WatchLogService
	postService     *services.PostService
	userService     *services.UserService
	redis           *redis.Client
}

// NewWatchLogHandler creates a new watch log handler.
func NewWatchLogHandler(db *sql.DB, redisClient *redis.Client) *WatchLogHandler {
	return &WatchLogHandler{
		watchLogService: services.NewWatchLogService(db, nil),
		postService:     services.NewPostService(db),
		userService:     services.NewUserService(db),
		redis:           redisClient,
	}
}

// LogWatch handles POST /api/v1/posts/{postId}/watch-log.
func (h *WatchLogHandler) LogWatch(w http.ResponseWriter, r *http.Request) {
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

	var req models.LogWatchRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if req.WatchedAt.IsZero() {
		writeError(r.Context(), w, http.StatusBadRequest, "WATCHED_AT_REQUIRED", "watched_at is required")
		return
	}

	notes := ""
	if req.Notes != nil {
		notes = *req.Notes
	}

	watchLog, err := h.watchLogService.LogWatchAt(r.Context(), userID, postID, req.Rating, notes, &req.WatchedAt)
	if err != nil {
		switch err.Error() {
		case "rating must be between 1 and 5":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_RATING", err.Error())
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		case "post is not a movie or series":
			writeError(r.Context(), w, http.StatusBadRequest, "POST_NOT_MOVIE_OR_SERIES", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "WATCH_LOG_CREATE_FAILED", "Failed to log watch")
		}
		return
	}

	publishCtx, cancel := publishContext()
	username := ""
	if user, err := h.userService.GetUserByID(publishCtx, userID); err == nil {
		username = user.Username
	} else {
		observability.LogWarn(publishCtx, "failed to load user for movie_watched event",
			"user_id", userID.String(),
			"post_id", postID.String(),
			"error", err.Error(),
		)
	}

	eventData := movieWatchedEventData{
		PostID:   postID,
		UserID:   userID,
		Username: username,
		Rating:   watchLog.Rating,
	}
	_ = publishEvent(publishCtx, h.redis, formatChannel(postPrefix, postID), "movie_watched", eventData)
	if sectionID, err := h.postService.GetSectionIDByPostID(publishCtx, postID); err == nil {
		_ = publishEvent(publishCtx, h.redis, formatChannel(sectionPrefix, sectionID), "movie_watched", eventData)
	}
	cancel()

	observability.LogInfo(r.Context(), "watch log created",
		"watch_log_id", watchLog.ID.String(),
		"user_id", userID.String(),
		"post_id", postID.String(),
		"rating", strconv.Itoa(watchLog.Rating),
	)

	response := models.CreateWatchLogResponse{WatchLog: *watchLog}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode create watch log response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusCreated,
			Err:        err,
		})
	}
}

// UpdateWatchLog handles PUT /api/v1/posts/{postId}/watch-log.
func (h *WatchLogHandler) UpdateWatchLog(w http.ResponseWriter, r *http.Request) {
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

	var req models.UpdateWatchLogRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	watchLog, err := h.watchLogService.UpdateWatchLog(r.Context(), userID, postID, req.Rating, req.Notes)
	if err != nil {
		switch err.Error() {
		case "no fields to update":
			writeError(r.Context(), w, http.StatusBadRequest, "NO_FIELDS_TO_UPDATE", err.Error())
		case "rating must be between 1 and 5":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_RATING", err.Error())
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		case "post is not a movie or series":
			writeError(r.Context(), w, http.StatusBadRequest, "POST_NOT_MOVIE_OR_SERIES", err.Error())
		case "watch log not found":
			writeError(r.Context(), w, http.StatusNotFound, "WATCH_LOG_NOT_FOUND", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "WATCH_LOG_UPDATE_FAILED", "Failed to update watch log")
		}
		return
	}

	observability.LogInfo(r.Context(), "watch log updated",
		"watch_log_id", watchLog.ID.String(),
		"user_id", userID.String(),
		"post_id", postID.String(),
	)

	response := models.UpdateWatchLogResponse{WatchLog: *watchLog}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode update watch log response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// RemoveWatchLog handles DELETE /api/v1/posts/{postId}/watch-log.
func (h *WatchLogHandler) RemoveWatchLog(w http.ResponseWriter, r *http.Request) {
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

	if err := h.watchLogService.RemoveWatchLog(r.Context(), userID, postID); err != nil {
		switch err.Error() {
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		case "post is not a movie or series":
			writeError(r.Context(), w, http.StatusBadRequest, "POST_NOT_MOVIE_OR_SERIES", err.Error())
		case "watch log not found":
			writeError(r.Context(), w, http.StatusNotFound, "WATCH_LOG_NOT_FOUND", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "WATCH_LOG_DELETE_FAILED", "Failed to remove watch log")
		}
		return
	}

	publishCtx, cancel := publishContext()
	eventData := movieWatchRemovedEventData{PostID: postID, UserID: userID}
	_ = publishEvent(publishCtx, h.redis, formatChannel(postPrefix, postID), "movie_watch_removed", eventData)
	if sectionID, err := h.postService.GetSectionIDByPostID(publishCtx, postID); err == nil {
		_ = publishEvent(publishCtx, h.redis, formatChannel(sectionPrefix, sectionID), "movie_watch_removed", eventData)
	}
	cancel()

	observability.LogInfo(r.Context(), "watch log removed",
		"user_id", userID.String(),
		"post_id", postID.String(),
	)

	w.WriteHeader(http.StatusNoContent)
}

// GetPostWatchLogs handles GET /api/v1/posts/{postId}/watch-logs.
func (h *WatchLogHandler) GetPostWatchLogs(w http.ResponseWriter, r *http.Request) {
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

	info, err := h.watchLogService.GetPostWatchLogs(r.Context(), postID, &userID)
	if err != nil {
		switch err.Error() {
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		case "post is not a movie or series":
			writeError(r.Context(), w, http.StatusBadRequest, "POST_NOT_MOVIE_OR_SERIES", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "GET_WATCH_LOGS_FAILED", "Failed to get watch logs")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode get post watch logs response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// GetMyWatchLogs handles GET /api/v1/me/watch-logs.
func (h *WatchLogHandler) GetMyWatchLogs(w http.ResponseWriter, r *http.Request) {
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

	logs, nextCursor, err := h.watchLogService.GetUserWatchLogs(r.Context(), userID, limit, cursorPtr)
	if err != nil {
		switch err.Error() {
		case "user not found":
			writeError(r.Context(), w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
		case "invalid cursor":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_CURSOR", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "GET_WATCH_LOGS_FAILED", "Failed to get watch logs")
		}
		return
	}

	response := models.ListWatchLogsResponse{
		WatchLogs:  logs,
		NextCursor: nextCursor,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode list watch logs response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}
