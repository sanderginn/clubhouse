package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/services"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestLoginSuccessLogsAuthEvent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() { testutil.CleanupRedis(t) })

	password := "Password1234!"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	userID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'loginuser', 'loginuser@example.com', $2, false, now(), now())
	`, userID, string(hash))
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	handler := NewAuthHandler(db, redisClient)

	reqBody := `{"username":"loginuser","password":"` + password + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "203.0.113.10:1234"
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var identifier, ipAddress, userAgent string
	err = db.QueryRow(`
		SELECT identifier, ip_address, user_agent
		FROM auth_events
		WHERE event_type = 'login_success' AND user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(&identifier, &ipAddress, &userAgent)
	if err != nil {
		t.Fatalf("failed to query auth event: %v", err)
	}

	if identifier != "loginuser" {
		t.Errorf("expected identifier loginuser, got %s", identifier)
	}
	if ipAddress != "203.0.113.10" {
		t.Errorf("expected ip address 203.0.113.10, got %s", ipAddress)
	}
	if userAgent != "test-agent" {
		t.Errorf("expected user agent test-agent, got %s", userAgent)
	}
}

func TestLoginFailureLogsAuthEvent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() { testutil.CleanupRedis(t) })

	handler := NewAuthHandler(db, redisClient)

	reqBody := `{"username":"missinguser","password":"wrongpassword"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "203.0.113.11:1234"
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, w.Code, w.Body.String())
	}

	var identifier, ipAddress, userAgent string
	err := db.QueryRow(`
		SELECT identifier, ip_address, user_agent
		FROM auth_events
		WHERE event_type = 'login_failure' AND identifier = 'missinguser'
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(&identifier, &ipAddress, &userAgent)
	if err != nil {
		t.Fatalf("failed to query auth event: %v", err)
	}

	if identifier != "missinguser" {
		t.Errorf("expected identifier missinguser, got %s", identifier)
	}
	if ipAddress != "203.0.113.11" {
		t.Errorf("expected ip address 203.0.113.11, got %s", ipAddress)
	}
	if userAgent != "test-agent" {
		t.Errorf("expected user agent test-agent, got %s", userAgent)
	}
}

func TestLogoutLogsAuthEvent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() { testutil.CleanupRedis(t) })

	userID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'logoutuser', 'logoutuser@example.com', '$2a$12$test', false, now(), now())
	`, userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	sessionService := services.NewSessionService(redisClient)
	session, err := sessionService.CreateSession(context.Background(), userID, "logoutuser", false)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	handler := NewAuthHandler(db, redisClient)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: session.ID})
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "203.0.113.12:1234"
	w := httptest.NewRecorder()

	handler.Logout(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var identifier, ipAddress, userAgent string
	err = db.QueryRow(`
		SELECT identifier, ip_address, user_agent
		FROM auth_events
		WHERE event_type = 'logout' AND user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(&identifier, &ipAddress, &userAgent)
	if err != nil {
		t.Fatalf("failed to query auth event: %v", err)
	}

	if identifier != "logoutuser" {
		t.Errorf("expected identifier logoutuser, got %s", identifier)
	}
	if ipAddress != "203.0.113.12" {
		t.Errorf("expected ip address 203.0.113.12, got %s", ipAddress)
	}
	if userAgent != "test-agent" {
		t.Errorf("expected user agent test-agent, got %s", userAgent)
	}
}

func TestAuthEventsListReturnsJson(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewAdminHandler(db, nil)

	_, err := db.Exec(`
		INSERT INTO auth_events (id, event_type, created_at)
		VALUES (gen_random_uuid(), 'login_success', now())
	`)
	if err != nil {
		t.Fatalf("failed to create auth event: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/auth-events", nil)
	w := httptest.NewRecorder()

	handler.GetAuthEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := response["events"]; !ok {
		t.Fatalf("expected events field in response")
	}
}
