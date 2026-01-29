package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

// UserHandler handles user endpoints
type UserHandler struct {
	db          *sql.DB
	userService *services.UserService
	postService *services.PostService
	totpService *services.TOTPService
}

// NewUserHandler creates a new user handler
func NewUserHandler(db *sql.DB) *UserHandler {
	return &UserHandler{
		db:          db,
		userService: services.NewUserService(db),
		postService: services.NewPostService(db),
		totpService: services.NewTOTPService(db),
	}
}

// GetProfile handles GET /api/v1/users/{id}
func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "User ID is required")
		return
	}

	userIDStr := pathParts[4]
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID format")
		return
	}

	profile, err := h.userService.GetUserProfile(r.Context(), userID)
	if err != nil {
		if err.Error() == "user not found" {
			writeError(r.Context(), w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_PROFILE_FAILED", "Failed to get user profile")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(profile); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode profile response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// GetUserPosts handles GET /api/v1/users/{id}/posts
func (h *UserHandler) GetUserPosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	// Extract user ID from URL path: /api/v1/users/{id}/posts
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 6 || pathParts[5] != "posts" {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "User ID is required")
		return
	}

	userIDStr := pathParts[4]
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID format")
		return
	}

	// Parse query parameters
	cursor := r.URL.Query().Get("cursor")
	limitStr := r.URL.Query().Get("limit")

	limit := 20
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Clamp limit to reasonable range
	if limit > 100 {
		limit = 100
	}

	// Get user posts from service
	var cursorPtr *string
	if cursor != "" {
		cursorPtr = &cursor
	}

	viewerID, _ := middleware.GetUserIDFromContext(r.Context())
	feed, err := h.postService.GetPostsByUserID(r.Context(), userID, cursorPtr, limit, viewerID)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_USER_POSTS_FAILED", "Failed to get user posts")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(feed); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode user posts response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// GetUserComments handles GET /api/v1/users/{id}/comments
func (h *UserHandler) GetUserComments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	// Extract user ID from URL path: /api/v1/users/{id}/comments
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 6 || pathParts[5] != "comments" {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "User ID is required")
		return
	}

	userIDStr := pathParts[4]
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID format")
		return
	}

	// Parse query parameters
	cursor := r.URL.Query().Get("cursor")
	limitStr := r.URL.Query().Get("limit")

	limit := 20
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Clamp limit to reasonable range
	if limit > 100 {
		limit = 100
	}

	var cursorPtr *string
	if cursor != "" {
		cursorPtr = &cursor
	}

	// Get user comments from service
	response, err := h.userService.GetUserComments(r.Context(), userID, cursorPtr, limit)
	if err != nil {
		if err.Error() == "user not found" {
			writeError(r.Context(), w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_USER_COMMENTS_FAILED", "Failed to get user comments")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode user comments response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// UpdateMe handles PATCH /api/v1/users/me
func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PATCH requests are allowed")
		return
	}

	// Get user ID from context (set by auth middleware)
	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Parse request body
	var req models.UpdateUserRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Update profile
	response, err := h.userService.UpdateProfile(r.Context(), userID, &req)
	if err != nil {
		switch err.Error() {
		case "user not found":
			writeError(r.Context(), w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
		case "at least one field (bio or profile_picture_url) is required":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		case "invalid profile picture URL":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_URL", err.Error())
		case "profile picture URL must use http or https scheme":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_URL_SCHEME", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "UPDATE_FAILED", "Failed to update profile")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode update me response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// EnrollMFA handles POST /api/v1/users/me/mfa/enable
func (h *UserHandler) EnrollMFA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	if h.totpService == nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "TOTP_UNAVAILABLE", "TOTP service unavailable")
		return
	}

	session, err := middleware.GetUserFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	enrollment, err := h.totpService.EnrollUser(r.Context(), session.UserID, session.Username)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrTOTPKeyMissing), errors.Is(err, services.ErrTOTPKeyInvalid):
			writeError(r.Context(), w, http.StatusInternalServerError, "TOTP_CONFIG_MISSING", "TOTP configuration missing")
		case errors.Is(err, services.ErrTOTPAlreadyEnabled):
			writeError(r.Context(), w, http.StatusConflict, "TOTP_ALREADY_ENABLED", "TOTP already enabled")
		case errors.Is(err, services.ErrTOTPUserNotFound):
			writeError(r.Context(), w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "TOTP_ENROLL_FAILED", "Failed to enroll TOTP")
		}
		return
	}

	response := models.TOTPEnrollResponse{
		Secret:     enrollment.Secret,
		OtpAuthURL: enrollment.URL,
		Message:    "TOTP enrollment created",
	}
	h.logUserAudit(r.Context(), "enroll_mfa", session.UserID, map[string]interface{}{
		"method": "totp",
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode totp enroll response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// VerifyMFA handles POST /api/v1/users/me/mfa/verify
func (h *UserHandler) VerifyMFA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	if h.totpService == nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "TOTP_UNAVAILABLE", "TOTP service unavailable")
		return
	}

	session, err := middleware.GetUserFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	var req models.TOTPVerifyRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	backupCodes, err := services.GenerateBackupCodes()
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "TOTP_BACKUP_FAILED", "Failed to generate backup codes")
		return
	}

	if err := h.totpService.VerifyUser(r.Context(), session.UserID, req.Code); err != nil {
		switch {
		case errors.Is(err, services.ErrTOTPRequired):
			writeError(r.Context(), w, http.StatusBadRequest, "TOTP_REQUIRED", "TOTP code required")
		case errors.Is(err, services.ErrTOTPInvalid):
			writeError(r.Context(), w, http.StatusUnauthorized, "INVALID_TOTP", "Invalid TOTP code")
		case errors.Is(err, services.ErrTOTPNotEnrolled):
			writeError(r.Context(), w, http.StatusConflict, "TOTP_NOT_ENROLLED", "TOTP enrollment required")
		case errors.Is(err, services.ErrTOTPAlreadyEnabled):
			writeError(r.Context(), w, http.StatusConflict, "TOTP_ALREADY_ENABLED", "TOTP already enabled")
		case errors.Is(err, services.ErrTOTPUserNotFound):
			writeError(r.Context(), w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		case errors.Is(err, services.ErrTOTPKeyMissing), errors.Is(err, services.ErrTOTPKeyInvalid):
			writeError(r.Context(), w, http.StatusInternalServerError, "TOTP_CONFIG_MISSING", "TOTP configuration missing")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "TOTP_VERIFY_FAILED", "Failed to verify TOTP")
		}
		return
	}

	response := models.TOTPVerifyResponse{
		Message:     "TOTP enabled",
		BackupCodes: backupCodes,
	}
	h.logUserAudit(r.Context(), "enable_mfa", session.UserID, map[string]interface{}{
		"method": "totp",
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode totp verify response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// DisableMFA handles POST /api/v1/users/me/mfa/disable
func (h *UserHandler) DisableMFA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	if h.totpService == nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "TOTP_UNAVAILABLE", "TOTP service unavailable")
		return
	}

	session, err := middleware.GetUserFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	var req models.TOTPVerifyRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := h.totpService.DisableUser(r.Context(), session.UserID, req.Code); err != nil {
		switch {
		case errors.Is(err, services.ErrTOTPRequired):
			writeError(r.Context(), w, http.StatusBadRequest, "TOTP_REQUIRED", "TOTP code required")
		case errors.Is(err, services.ErrTOTPInvalid):
			writeError(r.Context(), w, http.StatusUnauthorized, "INVALID_TOTP", "Invalid TOTP code")
		case errors.Is(err, services.ErrTOTPNotEnabled):
			writeError(r.Context(), w, http.StatusConflict, "TOTP_NOT_ENABLED", "TOTP is not enabled")
		case errors.Is(err, services.ErrTOTPNotEnrolled):
			writeError(r.Context(), w, http.StatusConflict, "TOTP_NOT_ENROLLED", "TOTP enrollment required")
		case errors.Is(err, services.ErrTOTPUserNotFound):
			writeError(r.Context(), w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		case errors.Is(err, services.ErrTOTPKeyMissing), errors.Is(err, services.ErrTOTPKeyInvalid):
			writeError(r.Context(), w, http.StatusInternalServerError, "TOTP_CONFIG_MISSING", "TOTP configuration missing")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "TOTP_DISABLE_FAILED", "Failed to disable TOTP")
		}
		return
	}

	response := models.TOTPDisableResponse{
		Message: "TOTP disabled",
	}
	h.logUserAudit(r.Context(), "disable_mfa", session.UserID, map[string]interface{}{
		"method": "totp",
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode totp disable response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

func (h *UserHandler) logUserAudit(ctx context.Context, action string, targetUserID uuid.UUID, metadata map[string]interface{}) {
	if h == nil || h.db == nil {
		return
	}

	userID, err := middleware.GetUserIDFromContext(ctx)
	if err != nil {
		return
	}

	auditService := services.NewAuditService(h.db)
	if err := auditService.LogAuditWithMetadata(ctx, action, userID, targetUserID, metadata); err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message:    "failed to create audit log",
			Code:       "AUDIT_LOG_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// GetMySectionSubscriptions handles GET /api/v1/users/me/section-subscriptions
func (h *UserHandler) GetMySectionSubscriptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	subscriptions, err := h.userService.GetSectionSubscriptions(r.Context(), userID)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "GET_SECTION_SUBSCRIPTIONS_FAILED", "Failed to get section subscriptions")
		return
	}

	response := models.GetSectionSubscriptionsResponse{
		SectionSubscriptions: subscriptions,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode update subscription response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// UpdateMySectionSubscription handles PATCH /api/v1/users/me/section-subscriptions/{sectionId}
func (h *UserHandler) UpdateMySectionSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PATCH requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 7 || pathParts[5] != "section-subscriptions" {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Section ID is required")
		return
	}

	sectionIDStr := pathParts[6]
	sectionID, err := uuid.Parse(sectionIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_SECTION_ID", "Invalid section ID format")
		return
	}

	var req models.UpdateSectionSubscriptionRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	if req.OptedOut == nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "opted_out is required")
		return
	}

	response, err := h.userService.UpdateSectionSubscription(r.Context(), userID, sectionID, *req.OptedOut)
	if err != nil {
		switch err.Error() {
		case "section not found":
			writeError(r.Context(), w, http.StatusNotFound, "SECTION_NOT_FOUND", "Section not found")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "UPDATE_SECTION_SUBSCRIPTION_FAILED", "Failed to update section subscription")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode update subscriptions response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}
