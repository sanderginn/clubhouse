package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	userService *services.UserService
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *sql.DB) *AuthHandler {
	return &AuthHandler{
		userService: services.NewUserService(db),
	}
}

// Register handles user registration
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	user, err := h.userService.RegisterUser(r.Context(), &req)
	if err != nil {
		// Determine appropriate error code and status
		switch err.Error() {
		case "username is required":
			writeError(w, http.StatusBadRequest, "USERNAME_REQUIRED", err.Error())
		case "username must be between 3 and 50 characters":
			writeError(w, http.StatusBadRequest, "INVALID_USERNAME_LENGTH", err.Error())
		case "username can only contain alphanumeric characters and underscores":
			writeError(w, http.StatusBadRequest, "INVALID_USERNAME_FORMAT", err.Error())
		case "email is required":
			writeError(w, http.StatusBadRequest, "EMAIL_REQUIRED", err.Error())
		case "invalid email format":
			writeError(w, http.StatusBadRequest, "INVALID_EMAIL", err.Error())
		case "password must be at least 8 characters":
			writeError(w, http.StatusBadRequest, "INVALID_PASSWORD_LENGTH", err.Error())
		case "password must contain uppercase, lowercase, and numeric characters":
			writeError(w, http.StatusBadRequest, "INVALID_PASSWORD_STRENGTH", err.Error())
		case "username already exists":
			writeError(w, http.StatusConflict, "USERNAME_EXISTS", err.Error())
		case "email already exists":
			writeError(w, http.StatusConflict, "EMAIL_EXISTS", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "REGISTRATION_FAILED", "Failed to register user")
		}
		return
	}

	response := models.RegisterResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Message:  "Registration successful. Please wait for admin approval.",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// writeError is a helper to write error responses
func writeError(w http.ResponseWriter, statusCode int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(models.ErrorResponse{
		Error: message,
		Code:  code,
	})
}
