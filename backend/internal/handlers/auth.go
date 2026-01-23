package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
)

type authRateLimiter interface {
	Allow(ctx context.Context, ip string, identifiers []string) (bool, error)
}

type authFailureTracker interface {
	IsLocked(ctx context.Context, ip string, identifiers []string) (bool, time.Duration, error)
	RegisterFailure(ctx context.Context, ip string, identifiers []string) (bool, time.Duration, error)
	Reset(ctx context.Context, ip string, identifiers []string) error
}

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	userService    *services.UserService
	sessionService *services.SessionService
	rateLimiter    authRateLimiter
	failureTracker authFailureTracker
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *sql.DB, redis *redis.Client) *AuthHandler {
	return &AuthHandler{
		userService:    services.NewUserService(db),
		sessionService: services.NewSessionService(redis),
		rateLimiter:    services.NewAuthRateLimiter(redis),
		failureTracker: services.NewAuthFailureTracker(redis),
	}
}

// Register handles user registration
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	clientIP := getClientIP(r)

	var req models.RegisterRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if !h.checkRateLimit(r.Context(), w, clientIP, filterIdentifiers(req.Username, req.Email)) {
		return
	}

	user, err := h.userService.RegisterUser(r.Context(), &req)
	if err != nil {
		// Determine appropriate error code and status
		switch err.Error() {
		case "username is required":
			writeError(r.Context(), w, http.StatusBadRequest, "USERNAME_REQUIRED", err.Error())
		case "username must be between 3 and 50 characters":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_USERNAME_LENGTH", err.Error())
		case "username can only contain alphanumeric characters and underscores":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_USERNAME_FORMAT", err.Error())
		case "invalid email format":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_EMAIL", err.Error())
		case "password must be at least 8 characters":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_PASSWORD_LENGTH", err.Error())
		case "password must contain uppercase, lowercase, and numeric characters":
			writeError(r.Context(), w, http.StatusBadRequest, "INVALID_PASSWORD_STRENGTH", err.Error())
		case "username already exists":
			writeError(r.Context(), w, http.StatusConflict, "USERNAME_EXISTS", err.Error())
		case "email already exists":
			writeError(r.Context(), w, http.StatusConflict, "EMAIL_EXISTS", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "REGISTRATION_FAILED", "Failed to register user")
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
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode register response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusCreated,
			Err:        err,
		})
	}
}

// Login handles user login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	clientIP := getClientIP(r)

	var req models.LoginRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request body too large")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	identifiers := filterIdentifiers(req.Username)

	if !h.checkRateLimit(r.Context(), w, clientIP, identifiers) {
		return
	}

	if !h.checkLockout(r.Context(), w, clientIP, identifiers) {
		return
	}

	user, err := h.userService.LoginUser(r.Context(), &req)
	if err != nil {
		// Determine appropriate error code and status
		switch err.Error() {
		case "username is required":
			writeError(r.Context(), w, http.StatusBadRequest, "USERNAME_REQUIRED", err.Error())
		case "password is required":
			writeError(r.Context(), w, http.StatusBadRequest, "PASSWORD_REQUIRED", err.Error())
		case "invalid username or password":
			locked, retryAfter, lockErr := h.registerLoginFailure(r.Context(), clientIP, identifiers)
			if lockErr != nil {
				observability.LogError(r.Context(), observability.ErrorLog{
					Message:    "failed to register login failure",
					Code:       "LOGIN_FAILURE_TRACK_FAILED",
					StatusCode: http.StatusUnauthorized,
					Err:        lockErr,
				})
			}
			if locked {
				writeLockoutResponse(r.Context(), w, retryAfter)
				return
			}
			writeError(r.Context(), w, http.StatusUnauthorized, "INVALID_CREDENTIALS", err.Error())
		case "user not approved":
			writeError(r.Context(), w, http.StatusForbidden, "USER_NOT_APPROVED", err.Error())
		default:
			writeError(r.Context(), w, http.StatusInternalServerError, "LOGIN_FAILED", "Failed to login")
		}
		return
	}

	if err := h.clearLoginFailures(r.Context(), clientIP, identifiers); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to reset login failures",
			Code:       "LOGIN_FAILURE_RESET_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}

	// Create session
	session, err := h.sessionService.CreateSession(r.Context(), user.ID, user.Username, user.IsAdmin)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "SESSION_CREATE_FAILED", "Failed to create session")
		return
	}

	secureCookie := isSecureRequest(r)

	// Set httpOnly secure cookie
	cookie := &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		MaxAge:   int(services.SessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   secureCookie,
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
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode login response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// GetMe returns the current authenticated user
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	// Get session_id from cookie
	cookie, err := r.Cookie("session_id")
	if err != nil {
		if err == http.ErrNoCookie {
			writeError(r.Context(), w, http.StatusUnauthorized, "NO_SESSION", "No active session found")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to read session cookie")
		return
	}

	// Get session from Redis
	session, err := h.sessionService.GetSession(r.Context(), cookie.Value)
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "INVALID_SESSION", "Session not found or expired")
		return
	}

	// Get user from database
	user, err := h.userService.GetUserByID(r.Context(), session.UserID)
	if err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "USER_NOT_FOUND", "Failed to retrieve user")
		return
	}

	response := models.MeResponse{
		ID:                user.ID,
		Username:          user.Username,
		Email:             user.Email,
		ProfilePictureUrl: user.ProfilePictureURL,
		Bio:               user.Bio,
		IsAdmin:           user.IsAdmin,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode me response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

// Logout handles user logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST requests are allowed")
		return
	}

	// Get session_id from cookie
	cookie, err := r.Cookie("session_id")
	if err != nil {
		if err == http.ErrNoCookie {
			writeError(r.Context(), w, http.StatusUnauthorized, "NO_SESSION", "No active session found")
			return
		}
		writeError(r.Context(), w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to read session cookie")
		return
	}

	// Delete session from Redis
	if err := h.sessionService.DeleteSession(r.Context(), cookie.Value); err != nil {
		writeError(r.Context(), w, http.StatusInternalServerError, "LOGOUT_FAILED", "Failed to logout")
		return
	}

	secureCookie := isSecureRequest(r)

	// Clear session cookie by setting MaxAge to -1
	cookie = &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)

	response := models.LogoutResponse{
		Message: "Logout successful",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		observability.LogError(r.Context(), observability.ErrorLog{
			Message:    "failed to encode logout response",
			Code:       "ENCODE_FAILED",
			StatusCode: http.StatusOK,
			Err:        err,
		})
	}
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (h *AuthHandler) checkRateLimit(ctx context.Context, w http.ResponseWriter, clientIP string, identifiers []string) bool {
	if h.rateLimiter == nil {
		return true
	}

	allowed, err := h.rateLimiter.Allow(ctx, clientIP, identifiers)
	if err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message:    "auth rate limit check failed",
			Code:       "RATE_LIMIT_CHECK_FAILED",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		})
		return true
	}

	if !allowed {
		writeError(ctx, w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many attempts. Please try again later.")
		return false
	}

	return true
}

func (h *AuthHandler) checkLockout(ctx context.Context, w http.ResponseWriter, clientIP string, identifiers []string) bool {
	if h.failureTracker == nil || len(identifiers) == 0 {
		return true
	}

	locked, retryAfter, err := h.failureTracker.IsLocked(ctx, clientIP, identifiers)
	if err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message:    "auth lockout check failed",
			Code:       "LOCKOUT_CHECK_FAILED",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		})
		return true
	}

	if locked {
		writeLockoutResponse(ctx, w, retryAfter)
		return false
	}

	return true
}

func (h *AuthHandler) registerLoginFailure(ctx context.Context, clientIP string, identifiers []string) (bool, time.Duration, error) {
	if h.failureTracker == nil || len(identifiers) == 0 {
		return false, 0, nil
	}

	return h.failureTracker.RegisterFailure(ctx, clientIP, identifiers)
}

func (h *AuthHandler) clearLoginFailures(ctx context.Context, clientIP string, identifiers []string) error {
	if h.failureTracker == nil || len(identifiers) == 0 {
		return nil
	}

	return h.failureTracker.Reset(ctx, clientIP, identifiers)
}

func writeLockoutResponse(ctx context.Context, w http.ResponseWriter, retryAfter time.Duration) {
	if retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
	}
	writeError(ctx, w, http.StatusTooManyRequests, "LOGIN_LOCKED", "Too many failed attempts. Please try again later.")
}

func filterIdentifiers(identifiers ...string) []string {
	if len(identifiers) == 0 {
		return nil
	}

	filtered := make([]string, 0, len(identifiers))
	for _, identifier := range identifiers {
		if strings.TrimSpace(identifier) == "" {
			continue
		}
		filtered = append(filtered, identifier)
	}

	return filtered
}
