package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
)

// NotificationHandler handles notification endpoints.
type NotificationHandler struct {
	notificationService *services.NotificationService
}

// NewNotificationHandler creates a new notification handler.
func NewNotificationHandler(db *sql.DB) *NotificationHandler {
	return &NotificationHandler{
		notificationService: services.NewNotificationService(db),
	}
}

// GetNotifications handles GET /api/v1/notifications.
func (h *NotificationHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
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
			writeError(w, http.StatusBadRequest, "INVALID_CURSOR", "Invalid cursor format")
		case "cursor not found":
			writeError(w, http.StatusBadRequest, "CURSOR_NOT_FOUND", "Cursor not found")
		default:
			writeError(w, http.StatusInternalServerError, "GET_NOTIFICATIONS_FAILED", "Failed to get notifications")
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
	json.NewEncoder(w).Encode(response)
}
