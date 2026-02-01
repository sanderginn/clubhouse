package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

type stubAuthRateLimiter struct {
	allowed bool
	err     error
	calls   int
	lastIP  string
	lastIDs []string
}

func (s *stubAuthRateLimiter) Allow(_ context.Context, ip string, identifiers []string) (bool, error) {
	s.calls++
	s.lastIP = ip
	s.lastIDs = identifiers
	return s.allowed, s.err
}

type stubAuthUserService struct {
	registerErr error
	loginErr    error
	userByID    *models.User
	getUserErr  error
}

func (s *stubAuthUserService) RegisterUser(_ context.Context, _ *models.RegisterRequest) (*models.User, error) {
	if s.registerErr != nil {
		return nil, s.registerErr
	}
	return &models.User{ID: uuid.New()}, nil
}

func (s *stubAuthUserService) LoginUser(_ context.Context, _ *models.LoginRequest) (*models.User, error) {
	if s.loginErr != nil {
		return nil, s.loginErr
	}
	return &models.User{ID: uuid.New()}, nil
}

func (s *stubAuthUserService) GetUserByID(_ context.Context, _ uuid.UUID) (*models.User, error) {
	if s.getUserErr != nil {
		return nil, s.getUserErr
	}
	if s.userByID != nil {
		return s.userByID, nil
	}
	return nil, errors.New("not implemented")
}

func TestLoginRateLimited(t *testing.T) {
	limiter := &stubAuthRateLimiter{allowed: false}
	handler := &AuthHandler{rateLimiter: limiter}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"TestUser","password":"Password123"}`))
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", w.Code)
	}

	var resp models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "RATE_LIMITED" {
		t.Fatalf("expected RATE_LIMITED code, got %s", resp.Code)
	}
}

func TestLoginGenericErrorForInvalidCredentials(t *testing.T) {
	handler := &AuthHandler{
		userService: &stubAuthUserService{loginErr: services.ErrInvalidCredentials},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"TestUser","password":"Password123"}`))
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", w.Code)
	}

	var resp models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "INVALID_CREDENTIALS" {
		t.Fatalf("expected INVALID_CREDENTIALS code, got %s", resp.Code)
	}
	if resp.Error != "Invalid username or password" {
		t.Fatalf("expected generic error message, got %s", resp.Error)
	}
}

func TestLoginUnapprovedUser(t *testing.T) {
	handler := &AuthHandler{
		userService: &stubAuthUserService{loginErr: services.ErrUserNotApproved},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"TestUser","password":"Password123"}`))
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", w.Code)
	}

	var resp models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "USER_NOT_APPROVED" {
		t.Fatalf("expected USER_NOT_APPROVED code, got %s", resp.Code)
	}
	if resp.Error != "Your account is awaiting admin approval." {
		t.Fatalf("expected approval message, got %s", resp.Error)
	}
}

func TestLoginSuspendedUser(t *testing.T) {
	handler := &AuthHandler{
		userService: &stubAuthUserService{loginErr: services.ErrUserSuspended},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"TestUser","password":"Password123"}`))
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", w.Code)
	}

	var resp models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "USER_SUSPENDED" {
		t.Fatalf("expected USER_SUSPENDED code, got %s", resp.Code)
	}
	if resp.Error != "Your account has been suspended." {
		t.Fatalf("expected suspended message, got %s", resp.Error)
	}
}

func TestLoginMFASetupRequired(t *testing.T) {
	services.ResetConfigServiceForTests()
	t.Cleanup(services.ResetConfigServiceForTests)

	required := true
	if _, err := services.GetConfigService().UpdateConfig(context.Background(), nil, &required, nil); err != nil {
		t.Fatalf("failed to enable mfa_required: %v", err)
	}

	handler := &AuthHandler{
		userService: &stubAuthUserService{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"TestUser","password":"Password123"}`))
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", w.Code)
	}

	var resp models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "MFA_SETUP_REQUIRED" {
		t.Fatalf("expected MFA_SETUP_REQUIRED code, got %s", resp.Code)
	}
	if !resp.MFARequired {
		t.Fatalf("expected mfa_required to be true")
	}
}

func TestGetMeReturnsNotFoundForMissingUser(t *testing.T) {
	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() { testutil.CleanupRedis(t) })

	sessionService := services.NewSessionService(redisClient)
	userID := uuid.New()
	session, err := sessionService.CreateSession(context.Background(), userID, "testuser", false)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	handler := NewAuthHandler(nil, redisClient)
	handler.userService = &stubAuthUserService{
		getUserErr: errors.New("user not found"),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: session.ID})
	w := httptest.NewRecorder()

	handler.GetMe(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}

	var resp models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "USER_NOT_FOUND" {
		t.Fatalf("expected USER_NOT_FOUND code, got %s", resp.Code)
	}
}

func TestGetMeReturnsServerErrorForLookupFailure(t *testing.T) {
	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() { testutil.CleanupRedis(t) })

	sessionService := services.NewSessionService(redisClient)
	userID := uuid.New()
	session, err := sessionService.CreateSession(context.Background(), userID, "testuser", false)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	handler := NewAuthHandler(nil, redisClient)
	handler.userService = &stubAuthUserService{
		getUserErr: errors.New("db failed"),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: session.ID})
	w := httptest.NewRecorder()

	handler.GetMe(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	var resp models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "USER_LOOKUP_FAILED" {
		t.Fatalf("expected USER_LOOKUP_FAILED code, got %s", resp.Code)
	}
}

func TestRegisterRateLimited(t *testing.T) {
	limiter := &stubAuthRateLimiter{allowed: false}
	handler := &AuthHandler{rateLimiter: limiter}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"username":"TestUser","email":"test@example.com","password":"Password123"}`))
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", w.Code)
	}

	var resp models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "RATE_LIMITED" {
		t.Fatalf("expected RATE_LIMITED code, got %s", resp.Code)
	}
}

func TestRegisterGenericConflictForExistingUser(t *testing.T) {
	tests := []struct {
		name        string
		registerErr error
	}{
		{
			name:        "username exists",
			registerErr: errors.New("username already exists"),
		},
		{
			name:        "email exists",
			registerErr: errors.New("email already exists"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &AuthHandler{
				userService: &stubAuthUserService{registerErr: tt.registerErr},
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"username":"TestUser","email":"test@example.com","password":"Password123"}`))
			w := httptest.NewRecorder()

			handler.Register(w, req)

			if w.Code != http.StatusConflict {
				t.Fatalf("expected status 409, got %d", w.Code)
			}

			var resp models.ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Code != "CONFLICT" {
				t.Fatalf("expected CONFLICT code, got %s", resp.Code)
			}
			if resp.Error != "Registration conflict." {
				t.Fatalf("expected generic error message, got %s", resp.Error)
			}
		})
	}
}

func TestRegisterCreatesAdminNotifications(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() { testutil.CleanupRedis(t) })

	adminID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'notifyadmin', 'notifyadmin@example.com', '$2a$12$test', true, now(), now())
	`, adminID)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	handler := NewAuthHandler(db, redisClient)
	handler.rateLimiter = &stubAuthRateLimiter{allowed: true}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"username":"NewUser","email":"newuser@example.com","password":"Password12345"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.RegisterResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	var count int
	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM notifications
		WHERE user_id = $1
		  AND type = 'user_registration_pending'
		  AND related_user_id = $2
	`, adminID, response.ID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query notifications: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 registration notification, got %d", count)
	}
}
