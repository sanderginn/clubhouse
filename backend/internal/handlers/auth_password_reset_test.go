package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestRedeemPasswordResetToken(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() { testutil.CleanupRedis(t) })

	handler := NewAuthHandler(db, redisClient)

	userID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'testuser', 'test@example.com', '$2a$12$oldpasswordhash', false, now(), now())
	`, userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	passwordResetService := services.NewPasswordResetService(redisClient)
	token, err := passwordResetService.GenerateToken(context.Background(), userID)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	reqBody := models.RedeemPasswordResetTokenRequest{
		Token:       token.Token,
		NewPassword: "newsecurepassword123",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/auth/password-reset/redeem", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RedeemPasswordResetToken(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.RedeemPasswordResetTokenResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if response.Message != "Password reset successful" {
		t.Errorf("expected success message, got %s", response.Message)
	}

	var passwordHash string
	err = db.QueryRow("SELECT password_hash FROM users WHERE id = $1", userID).Scan(&passwordHash)
	if err != nil {
		t.Fatalf("failed to get updated password hash: %v", err)
	}

	if passwordHash == "$2a$12$oldpasswordhash" {
		t.Error("expected password hash to be updated")
	}

	_, err = passwordResetService.GetToken(context.Background(), token.Token)
	if err != services.ErrPasswordResetTokenNotFound {
		t.Errorf("expected token to be deleted, got %v", err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM auth_events WHERE event_type = 'password_reset' AND user_id = $1", userID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query auth event count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 password_reset auth event, got %d", count)
	}
}

func TestRedeemPasswordResetTokenInvalidToken(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() { testutil.CleanupRedis(t) })

	handler := NewAuthHandler(db, redisClient)

	reqBody := models.RedeemPasswordResetTokenRequest{
		Token:       "invalid-token",
		NewPassword: "newsecurepassword123",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/auth/password-reset/redeem", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RedeemPasswordResetToken(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestRedeemPasswordResetTokenAlreadyUsed(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() { testutil.CleanupRedis(t) })

	handler := NewAuthHandler(db, redisClient)

	userID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'testuser2', 'test2@example.com', '$2a$12$oldpasswordhash', false, now(), now())
	`, userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	passwordResetService := services.NewPasswordResetService(redisClient)
	token, err := passwordResetService.GenerateToken(context.Background(), userID)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	if err := passwordResetService.MarkTokenAsUsed(context.Background(), token.Token); err != nil {
		t.Fatalf("failed to mark token as used: %v", err)
	}

	reqBody := models.RedeemPasswordResetTokenRequest{
		Token:       token.Token,
		NewPassword: "newsecurepassword123",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/auth/password-reset/redeem", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RedeemPasswordResetToken(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusConflict, w.Code, w.Body.String())
	}
}

func TestRedeemPasswordResetTokenWeakPassword(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() { testutil.CleanupRedis(t) })

	handler := NewAuthHandler(db, redisClient)

	userID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'testuser3', 'test3@example.com', '$2a$12$oldpasswordhash', false, now(), now())
	`, userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	passwordResetService := services.NewPasswordResetService(redisClient)
	token, err := passwordResetService.GenerateToken(context.Background(), userID)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	reqBody := models.RedeemPasswordResetTokenRequest{
		Token:       token.Token,
		NewPassword: "weak",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/auth/password-reset/redeem", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RedeemPasswordResetToken(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestRedeemPasswordResetTokenMethodNotAllowed(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() { testutil.CleanupRedis(t) })

	handler := NewAuthHandler(db, redisClient)

	req := httptest.NewRequest("GET", "/api/v1/auth/password-reset/redeem", nil)
	w := httptest.NewRecorder()

	handler.RedeemPasswordResetToken(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestRedeemPasswordResetTokenInvalidatesAllSessions(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() { testutil.CleanupRedis(t) })

	handler := NewAuthHandler(db, redisClient)

	userID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, is_admin, approved_at, created_at)
		VALUES ($1, 'testuser4', 'test4@example.com', '$2a$12$oldpasswordhash', false, now(), now())
	`, userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	sessionService := services.NewSessionService(redisClient)
	session, err := sessionService.CreateSession(context.Background(), userID, "testuser4", false)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	passwordResetService := services.NewPasswordResetService(redisClient)
	token, err := passwordResetService.GenerateToken(context.Background(), userID)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	reqBody := models.RedeemPasswordResetTokenRequest{
		Token:       token.Token,
		NewPassword: "newsecurepassword123",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/auth/password-reset/redeem", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RedeemPasswordResetToken(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	_, err = sessionService.GetSession(context.Background(), session.ID)
	if err != services.ErrSessionNotFound {
		t.Errorf("expected session to be invalidated, got %v", err)
	}
}
