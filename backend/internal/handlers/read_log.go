package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

// ReadLogHandler handles book read log endpoints.
type ReadLogHandler struct {
	readLogService *services.ReadLogService
}

// NewReadLogHandler creates a new read log handler.
func NewReadLogHandler(readLogService *services.ReadLogService) *ReadLogHandler {
	return &ReadLogHandler{
		readLogService: readLogService,
	}
}

// LogRead handles POST /api/v1/posts/{postId}/read.
func (h *ReadLogHandler) LogRead(w http.ResponseWriter, r *http.Request) {
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

	var req models.LogReadRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if req.Rating != nil && (*req.Rating < 1 || *req.Rating > 5) {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_RATING", "rating must be between 1 and 5")
		return
	}

	readLog, err := h.readLogService.LogRead(r.Context(), userID, postID, req.Rating)
	if err != nil {
		switch err.Error() {
		case "rating must be between 1 and 5":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_RATING", err.Error())
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		case "post is not a book":
			writeError(r.Context(), w, http.StatusBadRequest, "POST_NOT_BOOK", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "READ_LOG_CREATE_FAILED", "Failed to log read")
		}
		return
	}

	observability.LogInfo(r.Context(), "read log created",
		"read_log_id", readLog.ID.String(),
		"user_id", userID.String(),
		"post_id", postID.String(),
	)

	response := models.CreateReadLogResponse{ReadLog: *readLog}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode create read log response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusCreated,
			Err:        err,
		})
	}
}

// RemoveReadLog handles DELETE /api/v1/posts/{postId}/read.
func (h *ReadLogHandler) RemoveReadLog(w http.ResponseWriter, r *http.Request) {
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

	if err := h.readLogService.RemoveReadLog(r.Context(), userID, postID); err != nil {
		switch err.Error() {
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		case "post is not a book":
			writeError(r.Context(), w, http.StatusBadRequest, "POST_NOT_BOOK", err.Error())
		case "read log not found":
			writeError(r.Context(), w, http.StatusNotFound, "READ_LOG_NOT_FOUND", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "READ_LOG_DELETE_FAILED", "Failed to remove read log")
		}
		return
	}

	observability.LogInfo(r.Context(), "read log removed",
		"user_id", userID.String(),
		"post_id", postID.String(),
	)

	w.WriteHeader(http.StatusNoContent)
}

// UpdateReadLog handles PUT /api/v1/posts/{postId}/read.
func (h *ReadLogHandler) UpdateReadLog(w http.ResponseWriter, r *http.Request) {
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

	var req models.UpdateReadLogRequest
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
	if *req.Rating < 1 || *req.Rating > 5 {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_RATING", "rating must be between 1 and 5")
		return
	}

	readLog, err := h.readLogService.UpdateRating(r.Context(), userID, postID, *req.Rating)
	if err != nil {
		switch err.Error() {
		case "rating must be between 1 and 5":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_RATING", err.Error())
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		case "post is not a book":
			writeError(r.Context(), w, http.StatusBadRequest, "POST_NOT_BOOK", err.Error())
		case "read log not found":
			writeError(r.Context(), w, http.StatusNotFound, "READ_LOG_NOT_FOUND", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "READ_LOG_UPDATE_FAILED", "Failed to update read log")
		}
		return
	}

	observability.LogInfo(r.Context(), "read log updated",
		"read_log_id", readLog.ID.String(),
		"user_id", userID.String(),
		"post_id", postID.String(),
	)

	response := models.UpdateReadLogResponse{ReadLog: *readLog}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode update read log response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// GetPostReadLogs handles GET /api/v1/posts/{postId}/read.
func (h *ReadLogHandler) GetPostReadLogs(w http.ResponseWriter, r *http.Request) {
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

	info, err := h.readLogService.GetPostReadLogs(r.Context(), postID, &userID)
	if err != nil {
		switch err.Error() {
		case "post not found":
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", err.Error())
		case "post is not a book":
			writeError(r.Context(), w, http.StatusBadRequest, "POST_NOT_BOOK", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "GET_READ_LOGS_FAILED", "Failed to get read logs")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode get post read logs response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// GetReadHistory handles GET /api/v1/read-history.
func (h *ReadLogHandler) GetReadHistory(w http.ResponseWriter, r *http.Request) {
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

	readLogs, nextCursor, err := h.readLogService.GetUserReadHistory(r.Context(), userID, cursorPtr, limit)
	if err != nil {
		switch err.Error() {
		case "user not found":
			writeError(r.Context(), w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
		case "invalid cursor":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_CURSOR", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "GET_READ_HISTORY_FAILED", "Failed to get read history")
		}
		return
	}

	response := models.ListReadHistoryResponse{
		ReadLogs:   readLogs,
		NextCursor: nextCursor,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode list read history response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}
