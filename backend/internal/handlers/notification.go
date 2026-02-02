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

// NotificationHandler handles notification endpoints.
type NotificationHandler struct {
	notificationService *services.NotificationService
}

// NewNotificationHandler creates a new notification handler.
func NewNotificationHandler(db *sql.DB, redisClient *redis.Client, pushService *services.PushService) *NotificationHandler {
	return &NotificationHandler{
		notificationService: services.NewNotificationService(db, redisClient, pushService),
	}
}

// GetNotifications handles GET /api/v1/notifications.
func (h *NotificationHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limitStr := r.URL.Query().Get("limit")

	limit := 50
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	if limit > 100 {
		limit = 100
	}

	var cursorPtr *string
	if cursor != "" {
		cursorPtr = &cursor
	}

	notifications, nextCursor, hasMore, unreadCount, err := h.notificationService.GetNotifications(r.Context(), userID, limit, cursorPtr)
	if err != nil {
		switch err.Error() {
		case "invalid cursor":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_CURSOR", "Invalid cursor format")
		case "cursor not found":
			writeError(r.Context(), w, http.StatusBadRequest, "CURSOR_NOT_FOUND", "Cursor not found")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "GET_NOTIFICATIONS_FAILED", "Failed to get notifications")
		}
		return
	}

	response := models.GetNotificationsResponse{
		Notifications: notifications,
		Meta: models.NotificationMeta{
			Cursor:      nextCursor,
			HasMore:     hasMore,
			UnreadCount: unreadCount,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode notifications response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// MarkNotificationRead handles PATCH /api/v1/notifications/{id}.
func (h *NotificationHandler) MarkNotificationRead(w http.ResponseWriter, r *http.Request) {
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
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Notification ID is required")
		return
	}

	notificationIDStr := pathParts[4]
	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_NOTIFICATION_ID", "Invalid notification ID format")
		return
	}

	notification, err := h.notificationService.MarkNotificationRead(r.Context(), userID, notificationID)
	if err != nil {
		switch err.Error() {
		case "notification not found":
			writeError(r.Context(), w, http.StatusNotFound, "NOTIFICATION_NOT_FOUND", "Notification not found")
		case "forbidden":
			writeError(r.Context(), w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to update this notification")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "MARK_NOTIFICATION_READ_FAILED", "Failed to mark notification as read")
		}
		return
	}

	observability.RecordNotificationRead(r.Context(), "single", 1)

	response := models.UpdateNotificationResponse{
		Notification: *notification,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode update notification response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// MarkAllNotificationsRead handles PATCH /api/v1/notifications/read.
func (h *NotificationHandler) MarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PATCH requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	updatedCount, unreadCount, err := h.notificationService.MarkAllNotificationsRead(r.Context(), userID)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "MARK_ALL_NOTIFICATIONS_READ_FAILED", "Failed to mark notifications as read")
		return
	}

	observability.RecordNotificationRead(r.Context(), "all", updatedCount)

	response := models.MarkAllNotificationsReadResponse{
		UnreadCount: unreadCount,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode mark all notifications read response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}
