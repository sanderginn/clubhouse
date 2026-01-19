package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/services"
)

// UserHandler handles user endpoints
type UserHandler struct {
	userService *services.UserService
}

// NewUserHandler creates a new user handler
func NewUserHandler(db *sql.DB) *UserHandler {
	return &UserHandler{
		userService: services.NewUserService(db),
	}
}

// GetProfile handles GET /api/v1/users/{id}
func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "User ID is required")
		return
	}

	userIDStr := pathParts[4]
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID format")
		return
	}

	profile, err := h.userService.GetUserProfile(r.Context(), userID)
	if err != nil {
		if err.Error() == "user not found" {
			writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "GET_PROFILE_FAILED", "Failed to get user profile")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(profile)
}
