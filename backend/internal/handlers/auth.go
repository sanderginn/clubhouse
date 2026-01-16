package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	userService    *services.UserService
	sessionService *services.SessionService
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *sql.DB, redis *redis.Client) *AuthHandler {
	return &AuthHandler{
		userService:    services.NewUserService(db),
		sessionService: services.NewSessionService(redis),
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

// Login handles user login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	user, err := h.userService.LoginUser(r.Context(), &req)
	if err != nil {
		// Determine appropriate error code and status
		switch err.Error() {
		case "email is required":
			writeError(w, http.StatusBadRequest, "EMAIL_REQUIRED", err.Error())
		case "invalid email format":
			writeError(w, http.StatusBadRequest, "INVALID_EMAIL", err.Error())
		case "password is required":
			writeError(w, http.StatusBadRequest, "PASSWORD_REQUIRED", err.Error())
		case "invalid email or password":
			writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", err.Error())
		case "user not approved":
			writeError(w, http.StatusForbidden, "USER_NOT_APPROVED", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "LOGIN_FAILED", "Failed to login")
		}
		return
	}

	// Create session
	session, err := h.sessionService.CreateSession(r.Context(), user.ID, user.Username, user.IsAdmin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SESSION_CREATE_FAILED", "Failed to create session")
		return
	}

	// Set httpOnly secure cookie
	cookie := &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		MaxAge:   int(services.SessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   true, // Set to false in development, true in production
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)

	response := models.LoginResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		IsAdmin:  user.IsAdmin,
		Message:  "Login successful",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Logout handles user logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	// Get session_id from cookie
	cookie, err := r.Cookie("session_id")
	if err != nil {
		if err == http.ErrNoCookie {
			writeError(w, http.StatusUnauthorized, "NO_SESSION", "No active session found")
			return
		}
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to read session cookie")
		return
	}

	// Delete session from Redis
	if err := h.sessionService.DeleteSession(r.Context(), cookie.Value); err != nil {
		writeError(w, http.StatusInternalServerError, "LOGOUT_FAILED", "Failed to logout")
		return
	}

	// Clear session cookie by setting MaxAge to -1
	cookie = &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)

	response := models.LogoutResponse{
		Message: "Logout successful",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
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
