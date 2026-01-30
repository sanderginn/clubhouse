package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

// AdminHandler handles admin-specific endpoints
type AdminHandler struct {
	db                   *sql.DB
	userService          *services.UserService
	postService          *services.PostService
	commentService       *services.CommentService
	passwordResetService *services.PasswordResetService
	totpService          *services.TOTPService
	sessionService       *services.SessionService
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(db *sql.DB, redis *redis.Client) *AdminHandler {
	var sessionService *services.SessionService
	if redis != nil {
		sessionService = services.NewSessionService(redis)
	}

	return &AdminHandler{
		db:                   db,
		userService:          services.NewUserService(db),
		postService:          services.NewPostService(db),
		commentService:       services.NewCommentService(db),
		passwordResetService: services.NewPasswordResetService(redis),
		totpService:          services.NewTOTPService(db),
		sessionService:       sessionService,
	}
}

// ListPendingUsers returns all users pending admin approval
func (h *AdminHandler) ListPendingUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	pendingUsers, err := h.userService.GetPendingUsers(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "FETCH_FAILED", "Failed to fetch pending users")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(pendingUsers); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode pending users response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// ListApprovedUsers returns all approved users (admin only)
func (h *AdminHandler) ListApprovedUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	approvedUsers, err := h.userService.GetApprovedUsers(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "FETCH_FAILED", "Failed to fetch approved users")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(approvedUsers); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode approved users response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// ApproveUser approves a pending user (sets approved_at timestamp)
func (h *AdminHandler) ApproveUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PATCH requests are allowed")
		return
	}

	// Extract admin user ID from context
	adminUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Extract user ID from URL path: /admin/users/{id}/approve
	userIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/")
	userIDStr = strings.TrimSuffix(userIDStr, "/approve")

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID format")
		return
	}

	approveResponse, err := h.userService.ApproveUser(r.Context(), userID, adminUserID)
	if err != nil {
		// Determine appropriate error code and status
		switch err.Error() {
		case "user not found":
			writeError(r.Context(), w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
		case "user already approved":
			writeError(r.Context(), w, http.StatusConflict, "USER_ALREADY_APPROVED", err.Error())
		case "user has been deleted":
			writeError(r.Context(), w, http.StatusGone, "USER_DELETED", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "APPROVAL_FAILED", "Failed to approve user")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(approveResponse); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode approve user response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// PromoteUser promotes a user to admin (admin only)
func (h *AdminHandler) PromoteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	adminUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	userIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/")
	userIDStr = strings.TrimSuffix(userIDStr, "/promote")

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID format")
		return
	}

	if userID == adminUserID {
		writeError(r.Context(), w, http.StatusForbidden, "CANNOT_PROMOTE_SELF", "Cannot promote yourself")
		return
	}

	promoteResponse, err := h.userService.PromoteUserToAdmin(r.Context(), userID, adminUserID)
	if err != nil {
		switch err.Error() {
		case "user not found":
			writeError(r.Context(), w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
		case "user already admin":
			writeError(r.Context(), w, http.StatusConflict, "USER_ALREADY_ADMIN", err.Error())
		case "user has been deleted":
			writeError(r.Context(), w, http.StatusGone, "USER_DELETED", err.Error())
		case "cannot promote self":
			writeError(r.Context(), w, http.StatusForbidden, "CANNOT_PROMOTE_SELF", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "PROMOTION_FAILED", "Failed to promote user")
		}
		return
	}

	if h.sessionService != nil {
		if err := h.sessionService.UpdateUserAdminStatus(r.Context(), userID, true); err != nil {
			observability.LogError(r.Context(), observability.ErrorLog{
				Message:    "failed to update user sessions after promotion",
				Code:       "SESSION_UPDATE_FAILED",
				StatusCode: http.StatusInternalServerError,
				Err:        err,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(promoteResponse); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode promote user response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// SuspendUser suspends a user account (admin only)
func (h *AdminHandler) SuspendUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	adminUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	userIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/")
	userIDStr = strings.TrimSuffix(userIDStr, "/suspend")

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID format")
		return
	}

	var req models.SuspendUserRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		if !errors.Is(err, io.EOF) {
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
			return
		}
	}

	response, err := h.userService.SuspendUser(r.Context(), adminUserID, userID, req.Reason)
	if err != nil {
		switch err.Error() {
		case "user not found":
			writeError(r.Context(), w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
		case "user already suspended":
			writeError(r.Context(), w, http.StatusConflict, "USER_ALREADY_SUSPENDED", err.Error())
		case "user has been deleted":
			writeError(r.Context(), w, http.StatusGone, "USER_DELETED", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "SUSPEND_FAILED", "Failed to suspend user")
		}
		return
	}

	if h.sessionService != nil {
		if _, err := h.sessionService.DeleteAllSessionsForUser(r.Context(), userID); err != nil {
			observability.LogError(r.Context(), observability.ErrorLog{
				Message:    "failed to revoke user sessions after suspension",
				Code:       "SESSION_REVOKE_FAILED",
				StatusCode: http.StatusInternalServerError,
				Err:        err,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode suspend user response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// UnsuspendUser removes a user suspension (admin only)
func (h *AdminHandler) UnsuspendUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	adminUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	userIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/")
	userIDStr = strings.TrimSuffix(userIDStr, "/unsuspend")

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID format")
		return
	}

	response, err := h.userService.UnsuspendUser(r.Context(), adminUserID, userID)
	if err != nil {
		switch err.Error() {
		case "user not found":
			writeError(r.Context(), w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
		case "user not suspended":
			writeError(r.Context(), w, http.StatusConflict, "USER_NOT_SUSPENDED", err.Error())
		case "user has been deleted":
			writeError(r.Context(), w, http.StatusGone, "USER_DELETED", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "UNSUSPEND_FAILED", "Failed to unsuspend user")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode unsuspend user response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// RejectUser rejects a pending user (hard delete)
func (h *AdminHandler) RejectUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	// Extract admin user ID from context
	adminUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Extract user ID from URL path: /admin/users/{id}
	userIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/")

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID format")
		return
	}

	rejectResponse, err := h.userService.RejectUser(r.Context(), userID, adminUserID)
	if err != nil {
		// Determine appropriate error code and status
		switch err.Error() {
		case "user not found":
			writeError(r.Context(), w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
		case "cannot reject approved user":
			writeError(r.Context(), w, http.StatusConflict, "USER_ALREADY_APPROVED", "Cannot reject an already approved user")
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "REJECTION_FAILED", "Failed to reject user")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(rejectResponse); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode reject user response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// EnrollTOTP generates a TOTP secret for the admin user.
func (h *AdminHandler) EnrollTOTP(w http.ResponseWriter, r *http.Request) {
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

	enrollment, err := h.totpService.EnrollAdmin(r.Context(), session.UserID, session.Username)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrTOTPKeyMissing), errors.Is(err, services.ErrTOTPKeyInvalid):
			writeError(r.Context(), w, http.StatusInternalServerError, "TOTP_CONFIG_MISSING", "TOTP configuration missing")
		case errors.Is(err, services.ErrTOTPAlreadyEnabled):
			writeError(r.Context(), w, http.StatusConflict, "TOTP_ALREADY_ENABLED", "TOTP already enabled")
		case errors.Is(err, services.ErrTOTPUserNotFound):
			writeError(r.Context(), w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		case errors.Is(err, services.ErrTOTPAdminRequired):
			writeError(r.Context(), w, http.StatusForbidden, "ADMIN_REQUIRED", "Admin access required")
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
	h.logAdminAudit(r.Context(), "enroll_mfa", session.UserID, map[string]interface{}{
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

// VerifyTOTP verifies a TOTP code and enables MFA for the admin user.
func (h *AdminHandler) VerifyTOTP(w http.ResponseWriter, r *http.Request) {
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

	if err := h.totpService.VerifyAdmin(r.Context(), session.UserID, req.Code); err != nil {
		observability.RecordAuthTOTPVerification(r.Context(), "failure")
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

	observability.RecordAuthTOTPVerification(r.Context(), "success")

	response := models.TOTPVerifyResponse{
		Message: "TOTP enabled",
	}
	h.logAdminAudit(r.Context(), "enable_mfa", session.UserID, map[string]interface{}{
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

// HardDeletePost permanently deletes a post (admin only)
func (h *AdminHandler) HardDeletePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	// Extract admin user ID from context
	adminUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Extract post ID from URL path: /admin/posts/{id}
	postIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/posts/")

	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	err = h.postService.HardDeletePost(r.Context(), postID, adminUserID)
	if err != nil {
		if errors.Is(err, services.ErrPostNotFound) {
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "post not found")
		} else {
			writeError(r.Context(), w, http.StatusInternalServerError, "DELETE_FAILED", "Failed to delete post")
		}
		return
	}

	response := models.HardDeletePostResponse{
		ID:      postID,
		Message: "Post permanently deleted",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode hard delete post response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// HardDeleteComment permanently deletes a comment (admin only)
func (h *AdminHandler) HardDeleteComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	// Extract admin user ID from context
	adminUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Extract comment ID from URL path: /admin/comments/{id}
	commentIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/comments/")

	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_COMMENT_ID", "Invalid comment ID format")
		return
	}

	err = h.commentService.HardDeleteComment(r.Context(), commentID, adminUserID)
	if err != nil {
		if errors.Is(err, services.ErrCommentNotFound) {
			writeError(r.Context(), w, http.StatusNotFound, "COMMENT_NOT_FOUND", "comment not found")
		} else {
			writeError(r.Context(), w, http.StatusInternalServerError, "DELETE_FAILED", "Failed to delete comment")
		}
		return
	}

	response := models.HardDeleteCommentResponse{
		ID:      commentID,
		Message: "Comment permanently deleted",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode hard delete comment response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// AdminRestorePost restores a soft-deleted post (admin only)
func (h *AdminHandler) AdminRestorePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	// Extract admin user ID from context
	adminUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Extract post ID from URL path: /admin/posts/{id}/restore
	postIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/posts/")
	postIDStr = strings.TrimSuffix(postIDStr, "/restore")

	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	post, err := h.postService.AdminRestorePost(r.Context(), postID, adminUserID)
	if err != nil {
		if errors.Is(err, services.ErrPostNotFound) {
			writeError(r.Context(), w, http.StatusNotFound, "POST_NOT_FOUND", "post not found")
		} else if err.Error() == "post is not deleted" {
			writeError(r.Context(), w, http.StatusConflict, "POST_NOT_DELETED", "post is not deleted")
		} else {
			writeError(r.Context(), w, http.StatusInternalServerError, "RESTORE_FAILED", "Failed to restore post")
		}
		return
	}

	response := models.RestorePostResponse{
		Post: *post,
	}

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

// AdminRestoreComment restores a soft-deleted comment (admin only)
func (h *AdminHandler) AdminRestoreComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	// Extract admin user ID from context
	adminUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Extract comment ID from URL path: /admin/comments/{id}/restore
	commentIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/comments/")
	commentIDStr = strings.TrimSuffix(commentIDStr, "/restore")

	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_COMMENT_ID", "Invalid comment ID format")
		return
	}

	comment, err := h.commentService.AdminRestoreComment(r.Context(), commentID, adminUserID)
	if err != nil {
		if errors.Is(err, services.ErrCommentNotFound) {
			writeError(r.Context(), w, http.StatusNotFound, "COMMENT_NOT_FOUND", "comment not found")
		} else if err.Error() == "comment is not deleted" {
			writeError(r.Context(), w, http.StatusConflict, "COMMENT_NOT_DELETED", "comment is not deleted")
		} else {
			writeError(r.Context(), w, http.StatusInternalServerError, "RESTORE_FAILED", "Failed to restore comment")
		}
		return
	}

	response := models.RestoreCommentResponse{
		Comment: *comment,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode audit logs response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// UpdateConfigRequest represents the request body for updating config
type UpdateConfigRequest struct {
	LinkMetadataEnabled *bool `json:"linkMetadataEnabled"`
	MFARequired         *bool `json:"mfa_required"`
	MFARequiredAlt      *bool `json:"mfaRequired"`
}

// ConfigResponse wraps the config in a response object per API spec
type ConfigResponse struct {
	Config services.Config `json:"config"`
}

// GetConfig returns the current admin configuration
func (h *AdminHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	configService := services.GetConfigService()
	config := configService.GetConfig()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(ConfigResponse{Config: config}); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode config response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// UpdateConfig updates the admin configuration
func (h *AdminHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PATCH requests are allowed")
		return
	}

	var req UpdateConfigRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	configService := services.GetConfigService()
	previousConfig := configService.GetConfig()
	mfaRequired := req.MFARequired
	if mfaRequired == nil {
		mfaRequired = req.MFARequiredAlt
	}

	config, err := configService.UpdateConfig(r.Context(), req.LinkMetadataEnabled, mfaRequired)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "CONFIG_UPDATE_FAILED", "Failed to update config")
		return
	}

	if req.LinkMetadataEnabled != nil && previousConfig.LinkMetadataEnabled != config.LinkMetadataEnabled {
		h.logAdminAudit(r.Context(), "toggle_link_metadata", uuid.Nil, map[string]interface{}{
			"setting":   "link_metadata_enabled",
			"old_value": previousConfig.LinkMetadataEnabled,
			"new_value": config.LinkMetadataEnabled,
		})
	}
	if mfaRequired != nil && previousConfig.MFARequired != config.MFARequired {
		h.logAdminAudit(r.Context(), "toggle_mfa_requirement", uuid.Nil, map[string]interface{}{
			"setting":   "mfa_required",
			"old_value": previousConfig.MFARequired,
			"new_value": config.MFARequired,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(ConfigResponse{Config: config}); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode config response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// GetAuditLogs returns audit logs with pagination
func (h *AdminHandler) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	// Parse query parameters for pagination
	limit := 50 // Default limit
	cursor := r.URL.Query().Get("cursor")
	var cursorTimestamp *time.Time
	var cursorID *uuid.UUID
	if cursor != "" {
		parts := strings.SplitN(cursor, "|", 2)
		parsedTime, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid cursor format")
			return
		}
		cursorTimestamp = &parsedTime
		if len(parts) == 2 {
			parsedID, err := uuid.Parse(parts[1])
			if err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid cursor format")
				return
			}
			cursorID = &parsedID
		}
	}

	actions := normalizeAuditActions(r.URL.Query()["action"])
	if actionList := r.URL.Query().Get("actions"); actionList != "" {
		actions = normalizeAuditActions(append(actions, strings.Split(actionList, ",")...))
	}

	var adminUserID *uuid.UUID
	adminUserIDParam := r.URL.Query().Get("admin_user_id")
	if adminUserIDParam != "" {
		parsedID, err := uuid.Parse(adminUserIDParam)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid admin_user_id")
			return
		}
		adminUserID = &parsedID
	}

	var targetUserID *uuid.UUID
	targetUserIDParam := r.URL.Query().Get("target_user_id")
	if targetUserIDParam != "" {
		parsedID, err := uuid.Parse(targetUserIDParam)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid target_user_id")
			return
		}
		targetUserID = &parsedID
	}

	startDate, startDateOnly, err := parseAuditDateParam(r.URL.Query().Get("start"))
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid start date")
		return
	}

	endDate, endDateOnly, err := parseAuditDateParam(r.URL.Query().Get("end"))
	if err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid end date")
		return
	}

	if endDate != nil && endDateOnly {
		adjusted := endDate.Add(24 * time.Hour)
		endDate = &adjusted
	}
	if startDate != nil && startDateOnly {
		adjusted := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.UTC)
		startDate = &adjusted
	}

	if startDate != nil && endDate != nil && !startDate.Before(*endDate) {
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid date range")
		return
	}

	whereClauses := []string{`
		(
			$1::timestamp IS NULL
			OR ($2::uuid IS NULL AND a.created_at < $1)
			OR ($2 IS NOT NULL AND (a.created_at, a.id) < ($1, $2))
		)
	`}
	args := []interface{}{cursorTimestamp, cursorID}

	if len(actions) > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf("a.action = ANY($%d)", len(args)+1))
		args = append(args, pq.Array(actions))
	}

	if adminUserID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("a.admin_user_id = $%d", len(args)+1))
		args = append(args, *adminUserID)
	}

	if targetUserID != nil {
		placeholder := len(args) + 1
		whereClauses = append(whereClauses, fmt.Sprintf("(a.target_user_id = $%d OR a.related_user_id = $%d)", placeholder, placeholder))
		args = append(args, *targetUserID)
	}

	if startDate != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("a.created_at >= $%d", len(args)+1))
		args = append(args, *startDate)
	}

	if endDate != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("a.created_at < $%d", len(args)+1))
		args = append(args, *endDate)
	}

	query := `
		SELECT
			a.id,
			a.admin_user_id,
			admin.username,
			a.action,
			a.related_post_id,
			a.related_comment_id,
			a.related_user_id,
			related.username,
			a.target_user_id,
			target.username,
			a.metadata,
			a.created_at
		FROM audit_logs a
		LEFT JOIN users admin ON a.admin_user_id = admin.id
		LEFT JOIN users related ON a.related_user_id = related.id
		LEFT JOIN users target ON a.target_user_id = target.id
		WHERE ` + strings.Join(whereClauses, " AND ") + `
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT $` + fmt.Sprint(len(args)+1) + `
	`

	args = append(args, limit+1)
	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "FETCH_FAILED", "Failed to fetch audit logs")
		return
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		var adminUserID uuid.NullUUID
		var adminUsername sql.NullString
		var relatedUserID uuid.NullUUID
		var relatedUsername sql.NullString
		var targetUserID uuid.NullUUID
		var targetUsername sql.NullString
		var metadataBytes []byte
		err := rows.Scan(
			&log.ID,
			&adminUserID,
			&adminUsername,
			&log.Action,
			&log.RelatedPostID,
			&log.RelatedCommentID,
			&relatedUserID,
			&relatedUsername,
			&targetUserID,
			&targetUsername,
			&metadataBytes,
			&log.CreatedAt,
		)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "SCAN_FAILED", "Failed to parse audit log")
			return
		}
		if adminUserID.Valid {
			log.AdminUserID = &adminUserID.UUID
		}
		if adminUsername.Valid {
			log.AdminUsername = adminUsername.String
		}
		if relatedUserID.Valid {
			log.RelatedUserID = &relatedUserID.UUID
		}
		if relatedUsername.Valid {
			log.RelatedUsername = relatedUsername.String
		}
		if targetUserID.Valid {
			log.TargetUserID = &targetUserID.UUID
		}
		if targetUsername.Valid {
			log.TargetUsername = targetUsername.String
		}
		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &log.Metadata); err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "SCAN_FAILED", "Failed to parse audit log")
				return
			}
		}
		logs = append(logs, &log)
	}

	if err := rows.Err(); err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "FETCH_FAILED", "Failed to fetch audit logs")
		return
	}

	// Determine if there are more logs
	hasMore := len(logs) > limit
	if hasMore {
		logs = logs[:limit]
	}

	// Determine next cursor
	var nextCursor *string
	if hasMore && len(logs) > 0 {
		lastLog := logs[len(logs)-1]
		cursorStr := lastLog.CreatedAt.Format(time.RFC3339Nano) + "|" + lastLog.ID.String()
		nextCursor = &cursorStr
	}

	response := models.AuditLogsResponse{
		Logs:       logs,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode audit logs response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// GetAuditLogActions returns distinct audit log action types.
func (h *AdminHandler) GetAuditLogActions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `SELECT DISTINCT action FROM audit_logs ORDER BY action ASC`)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "FETCH_FAILED", "Failed to fetch audit log actions")
		return
	}
	defer rows.Close()

	var actions []string
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "SCAN_FAILED", "Failed to parse audit log actions")
			return
		}
		if strings.TrimSpace(action) == "" {
			continue
		}
		actions = append(actions, action)
	}

	if err := rows.Err(); err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "FETCH_FAILED", "Failed to fetch audit log actions")
		return
	}

	response := models.AuditLogActionsResponse{Actions: actions}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode audit log actions response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

func (h *AdminHandler) logAdminAudit(ctx context.Context, action string, targetUserID uuid.UUID, metadata map[string]interface{}) {
	if h == nil || h.db == nil {
		return
	}

	adminUserID, err := middleware.GetUserIDFromContext(ctx)
	if err != nil {
		return
	}

	auditService := services.NewAuditService(h.db)
	if err := auditService.LogAuditWithMetadata(ctx, action, adminUserID, targetUserID, metadata); err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message:    "failed to create audit log",
			Code:       "AUDIT_LOG_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

func parseAuditDateParam(value string) (*time.Time, bool, error) {
	if value == "" {
		return nil, false, nil
	}

	if strings.Contains(value, "T") {
		parsedTime, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			parsedTime, err = time.Parse(time.RFC3339, value)
		}
		if err != nil {
			return nil, false, err
		}
		return &parsedTime, false, nil
	}

	parsedDate, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, false, err
	}
	parsedDate = time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), 0, 0, 0, 0, time.UTC)
	return &parsedDate, true, nil
}

func normalizeAuditActions(values []string) []string {
	actions := make([]string, 0, len(values))
	seen := make(map[string]struct{})
	for _, value := range values {
		for _, action := range strings.Split(value, ",") {
			action = strings.TrimSpace(action)
			if action == "" {
				continue
			}
			if _, ok := seen[action]; ok {
				continue
			}
			seen[action] = struct{}{}
			actions = append(actions, action)
		}
	}
	return actions
}

// GetAuthEvents returns auth events with pagination
func (h *AdminHandler) GetAuthEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	limit := 50
	cursor := r.URL.Query().Get("cursor")
	var cursorTimestamp *time.Time
	var cursorID *uuid.UUID
	if cursor != "" {
		parts := strings.SplitN(cursor, "|", 2)
		parsedTime, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid cursor format")
			return
		}
		cursorTimestamp = &parsedTime
		if len(parts) == 2 {
			parsedID, err := uuid.Parse(parts[1])
			if err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid cursor format")
				return
			}
			cursorID = &parsedID
		}
	}

	query := `
		SELECT
			e.id, e.user_id, u.username, e.identifier, e.event_type,
			e.ip_address, e.user_agent, e.created_at
		FROM auth_events e
		LEFT JOIN users u ON e.user_id = u.id
		WHERE (
			$1::timestamp IS NULL
			OR ($2::uuid IS NULL AND e.created_at < $1)
			OR ($2 IS NOT NULL AND (e.created_at, e.id) < ($1, $2))
		)
		ORDER BY e.created_at DESC, e.id DESC
		LIMIT $3
	`

	rows, err := h.db.QueryContext(r.Context(), query, cursorTimestamp, cursorID, limit+1)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "FETCH_FAILED", "Failed to fetch auth events")
		return
	}
	defer rows.Close()

	var events []*models.AuthEvent
	for rows.Next() {
		var event models.AuthEvent
		var userID sql.NullString
		var username sql.NullString
		var identifier sql.NullString
		var ipAddress sql.NullString
		var userAgent sql.NullString
		if err := rows.Scan(
			&event.ID,
			&userID,
			&username,
			&identifier,
			&event.EventType,
			&ipAddress,
			&userAgent,
			&event.CreatedAt,
		); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "SCAN_FAILED", "Failed to parse auth event")
			return
		}

		if userID.Valid {
			parsed, err := uuid.Parse(userID.String)
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "SCAN_FAILED", "Failed to parse auth event")
				return
			}
			event.UserID = &parsed
		}
		if username.Valid {
			name := username.String
			event.Username = &name
		}
		if identifier.Valid {
			event.Identifier = identifier.String
		}
		if ipAddress.Valid {
			event.IPAddress = ipAddress.String
		}
		if userAgent.Valid {
			event.UserAgent = userAgent.String
		}

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "FETCH_FAILED", "Failed to fetch auth events")
		return
	}

	hasMore := len(events) > limit
	if hasMore {
		events = events[:limit]
	}

	var nextCursor *string
	if hasMore && len(events) > 0 {
		lastEvent := events[len(events)-1]
		cursorStr := lastEvent.CreatedAt.Format(time.RFC3339Nano) + "|" + lastEvent.ID.String()
		nextCursor = &cursorStr
	}

	response := models.AuthEventLogsResponse{
		Events:     events,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode auth events response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// GeneratePasswordResetToken generates a one-time password reset token for a user (admin only)
func (h *AdminHandler) GeneratePasswordResetToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	var req models.GeneratePasswordResetTokenRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Verify user exists and is approved
	user, err := h.userService.GetUserByID(r.Context(), req.UserID)
	if err != nil {
		if err.Error() == "user not found" {
			writeError(r.Context(), w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
			return
		}
		writeError(r.Context(), w, http.StatusInternalServerError, "USER_LOOKUP_FAILED", "Failed to lookup user")
		return
	}

	if user.ApprovedAt == nil {
		writeError(r.Context(), w, http.StatusBadRequest, "USER_NOT_APPROVED", "User is not approved")
		return
	}

	// Generate token
	token, err := h.passwordResetService.GenerateToken(r.Context(), req.UserID)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "TOKEN_GENERATION_FAILED", "Failed to generate password reset token")
		return
	}

	observability.RecordAuthPasswordReset(r.Context(), "token_generated")

	response := models.GeneratePasswordResetTokenResponse{
		Token:     token.Token,
		UserID:    token.UserID,
		ExpiresAt: token.ExpiresAt,
	}
	h.logAdminAudit(r.Context(), "generate_password_reset_token", req.UserID, nil)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode generate password reset token response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}
