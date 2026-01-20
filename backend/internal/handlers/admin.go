package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
)

// AdminHandler handles admin-specific endpoints
type AdminHandler struct {
	db             *sql.DB
	userService    *services.UserService
	postService    *services.PostService
	commentService *services.CommentService
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(db *sql.DB) *AdminHandler {
	return &AdminHandler{
		db:             db,
		userService:    services.NewUserService(db),
		postService:    services.NewPostService(db),
		commentService: services.NewCommentService(db),
	}
}

// ListPendingUsers returns all users pending admin approval
func (h *AdminHandler) ListPendingUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	pendingUsers, err := h.userService.GetPendingUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_FAILED", "Failed to fetch pending users")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(pendingUsers)
}

// ApproveUser approves a pending user (sets approved_at timestamp)
func (h *AdminHandler) ApproveUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PATCH requests are allowed")
		return
	}

	// Extract admin user ID from context
	adminUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Extract user ID from URL path: /admin/users/{id}/approve
	userIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/")
	userIDStr = strings.TrimSuffix(userIDStr, "/approve")

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID format")
		return
	}

	approveResponse, err := h.userService.ApproveUser(r.Context(), userID, adminUserID)
	if err != nil {
		// Determine appropriate error code and status
		switch err.Error() {
		case "user not found":
			writeError(w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
		case "user already approved":
			writeError(w, http.StatusConflict, "USER_ALREADY_APPROVED", err.Error())
		case "user has been deleted":
			writeError(w, http.StatusGone, "USER_DELETED", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "APPROVAL_FAILED", "Failed to approve user")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(approveResponse)
}

// RejectUser rejects a pending user (hard delete)
func (h *AdminHandler) RejectUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	// Extract admin user ID from context
	adminUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Extract user ID from URL path: /admin/users/{id}
	userIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/")

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID format")
		return
	}

	rejectResponse, err := h.userService.RejectUser(r.Context(), userID, adminUserID)
	if err != nil {
		// Determine appropriate error code and status
		switch err.Error() {
		case "user not found":
			writeError(w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
		case "cannot reject approved user":
			writeError(w, http.StatusConflict, "USER_ALREADY_APPROVED", "Cannot reject an already approved user")
		default:
			writeError(w, http.StatusInternalServerError, "REJECTION_FAILED", "Failed to reject user")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rejectResponse)
}

// HardDeletePost permanently deletes a post (admin only)
func (h *AdminHandler) HardDeletePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	// Extract admin user ID from context
	adminUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Extract post ID from URL path: /admin/posts/{id}
	postIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/posts/")

	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	err = h.postService.HardDeletePost(r.Context(), postID, adminUserID)
	if err != nil {
		if errors.Is(err, services.ErrPostNotFound) {
			writeError(w, http.StatusNotFound, "POST_NOT_FOUND", "post not found")
		} else {
			writeError(w, http.StatusInternalServerError, "DELETE_FAILED", "Failed to delete post")
		}
		return
	}

	response := models.HardDeletePostResponse{
		ID:      postID,
		Message: "Post permanently deleted",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// HardDeleteComment permanently deletes a comment (admin only)
func (h *AdminHandler) HardDeleteComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only DELETE requests are allowed")
		return
	}

	// Extract admin user ID from context
	adminUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Extract comment ID from URL path: /admin/comments/{id}
	commentIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/comments/")

	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_COMMENT_ID", "Invalid comment ID format")
		return
	}

	err = h.commentService.HardDeleteComment(r.Context(), commentID, adminUserID)
	if err != nil {
		if errors.Is(err, services.ErrCommentNotFound) {
			writeError(w, http.StatusNotFound, "COMMENT_NOT_FOUND", "comment not found")
		} else {
			writeError(w, http.StatusInternalServerError, "DELETE_FAILED", "Failed to delete comment")
		}
		return
	}

	response := models.HardDeleteCommentResponse{
		ID:      commentID,
		Message: "Comment permanently deleted",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// AdminRestorePost restores a soft-deleted post (admin only)
func (h *AdminHandler) AdminRestorePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	// Extract admin user ID from context
	adminUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Extract post ID from URL path: /admin/posts/{id}/restore
	postIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/posts/")
	postIDStr = strings.TrimSuffix(postIDStr, "/restore")

	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_POST_ID", "Invalid post ID format")
		return
	}

	post, err := h.postService.AdminRestorePost(r.Context(), postID, adminUserID)
	if err != nil {
		if errors.Is(err, services.ErrPostNotFound) {
			writeError(w, http.StatusNotFound, "POST_NOT_FOUND", "post not found")
		} else if err.Error() == "post is not deleted" {
			writeError(w, http.StatusConflict, "POST_NOT_DELETED", "post is not deleted")
		} else {
			writeError(w, http.StatusInternalServerError, "RESTORE_FAILED", "Failed to restore post")
		}
		return
	}

	response := models.RestorePostResponse{
		Post: *post,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// AdminRestoreComment restores a soft-deleted comment (admin only)
func (h *AdminHandler) AdminRestoreComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	// Extract admin user ID from context
	adminUserID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Extract comment ID from URL path: /admin/comments/{id}/restore
	commentIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/comments/")
	commentIDStr = strings.TrimSuffix(commentIDStr, "/restore")

	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_COMMENT_ID", "Invalid comment ID format")
		return
	}

	comment, err := h.commentService.AdminRestoreComment(r.Context(), commentID, adminUserID)
	if err != nil {
		if errors.Is(err, services.ErrCommentNotFound) {
			writeError(w, http.StatusNotFound, "COMMENT_NOT_FOUND", "comment not found")
		} else if err.Error() == "comment is not deleted" {
			writeError(w, http.StatusConflict, "COMMENT_NOT_DELETED", "comment is not deleted")
		} else {
			writeError(w, http.StatusInternalServerError, "RESTORE_FAILED", "Failed to restore comment")
		}
		return
	}

	response := models.RestoreCommentResponse{
		Comment: *comment,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// UpdateConfigRequest represents the request body for updating config
type UpdateConfigRequest struct {
	LinkMetadataEnabled *bool `json:"linkMetadataEnabled"`
}

// ConfigResponse wraps the config in a response object per API spec
type ConfigResponse struct {
	Config services.Config `json:"config"`
}

// GetConfig returns the current admin configuration
func (h *AdminHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	configService := services.GetConfigService()
	config := configService.GetConfig()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ConfigResponse{Config: config})
}

// UpdateConfig updates the admin configuration
func (h *AdminHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PATCH requests are allowed")
		return
	}

	var req UpdateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	configService := services.GetConfigService()
	config := configService.UpdateConfig(req.LinkMetadataEnabled)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ConfigResponse{Config: config})
}

// GetAuditLogs returns audit logs with pagination
func (h *AdminHandler) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	// Parse query parameters for pagination
	limit := 50 // Default limit
	cursor := r.URL.Query().Get("cursor")
	cursorTimestamp := ""
	cursorID := ""
	if cursor != "" {
		parts := strings.SplitN(cursor, "|", 2)
		cursorTimestamp = parts[0]
		if _, err := time.Parse(time.RFC3339Nano, cursorTimestamp); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid cursor format")
			return
		}
		if len(parts) == 2 {
			cursorID = parts[1]
			if _, err := uuid.Parse(cursorID); err != nil {
				writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid cursor format")
				return
			}
		}
	}

	// Query audit logs with admin username, ordered by created_at DESC
	query := `
		SELECT
			a.id, a.admin_user_id, u.username, a.action,
			a.related_post_id, a.related_comment_id, a.related_user_id, a.created_at
		FROM audit_logs a
		JOIN users u ON a.admin_user_id = u.id
		WHERE (
			$1 = ''
			OR ($2 = '' AND a.created_at < $1::timestamp)
			OR ($2 <> '' AND (a.created_at, a.id) < ($1::timestamp, $2::uuid))
		)
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT $3
	`

	rows, err := h.db.QueryContext(r.Context(), query, cursorTimestamp, cursorID, limit+1)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_FAILED", "Failed to fetch audit logs")
		return
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		err := rows.Scan(
			&log.ID, &log.AdminUserID, &log.AdminUsername, &log.Action,
			&log.RelatedPostID, &log.RelatedCommentID, &log.RelatedUserID, &log.CreatedAt,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "SCAN_FAILED", "Failed to parse audit log")
			return
		}
		logs = append(logs, &log)
	}

	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_FAILED", "Failed to fetch audit logs")
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
	json.NewEncoder(w).Encode(response)
}
