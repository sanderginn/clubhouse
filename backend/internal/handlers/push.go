package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

// PushHandler handles web push subscription endpoints.
type PushHandler struct {
	pushService *services.PushService
}

// NewPushHandler creates a push handler.
func NewPushHandler(db *sql.DB, pushService *services.PushService) *PushHandler {
	if pushService == nil {
		pushService = services.NewPushService(db)
	}
	return &PushHandler{pushService: pushService}
}

// GetVAPIDKey handles GET /api/v1/push/vapid-key.
func (h *PushHandler) GetVAPIDKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	publicKey, err := h.pushService.PublicKey()
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "VAPID_KEY_UNAVAILABLE", "VAPID public key is not configured")
		return
	}

	response := models.PushVAPIDKeyResponse{PublicKey: publicKey}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode vapid key response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// Subscribe handles POST /api/v1/push/subscribe.
func (h *PushHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	var req models.PushSubscriptionRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	req.Endpoint = strings.TrimSpace(req.Endpoint)
	req.Keys.Auth = strings.TrimSpace(req.Keys.Auth)
	req.Keys.P256dh = strings.TrimSpace(req.Keys.P256dh)

	if req.Endpoint == "" || req.Keys.Auth == "" || req.Keys.P256dh == "" {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Subscription endpoint and keys are required")
		return
	}

	if err := h.pushService.UpsertSubscription(r.Context(), userID, req); err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "SUBSCRIBE_FAILED", "Failed to store push subscription")
		return
	}

	observability.RecordPushSubscriptionCreated(r.Context())
	observability.LogInfo(r.Context(), "push subscription created", "user_id", userID.String())

	w.WriteHeader(http.StatusNoContent)
}

// Unsubscribe handles DELETE /api/v1/push/subscribe.
func (h *PushHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	if err := h.pushService.DeleteSubscriptions(r.Context(), userID); err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "UNSUBSCRIBE_FAILED", "Failed to remove push subscription")
		return
	}

	observability.RecordPushSubscriptionDeleted(r.Context())
	observability.LogInfo(r.Context(), "push subscription deleted", "user_id", userID.String())

	w.WriteHeader(http.StatusNoContent)
}
