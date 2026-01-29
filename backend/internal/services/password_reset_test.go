package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func setupPasswordResetTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	return testutil.GetTestRedis(t)
}

func TestPasswordResetService_GenerateToken(t *testing.T) {
	redisClient := setupPasswordResetTestRedis(t)
	defer testutil.CleanupRedis(t)

	service := NewPasswordResetService(redisClient)
	userID := uuid.New()

	token, err := service.GenerateToken(context.Background(), userID)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	if token.Token == "" {
		t.Error("expected non-empty token")
	}

	if token.UserID != userID {
		t.Errorf("expected user ID %v, got %v", userID, token.UserID)
	}

	if token.Used {
		t.Error("expected token to not be used")
	}

	if time.Until(token.ExpiresAt) > PasswordResetTokenDuration {
		t.Error("expected token expiration to be within duration")
	}
}

func TestPasswordResetService_GetToken(t *testing.T) {
	redisClient := setupPasswordResetTestRedis(t)
	defer testutil.CleanupRedis(t)

	service := NewPasswordResetService(redisClient)
	userID := uuid.New()

	token, err := service.GenerateToken(context.Background(), userID)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	retrievedToken, err := service.GetToken(context.Background(), token.Token)
	if err != nil {
		t.Fatalf("failed to get token: %v", err)
	}

	if retrievedToken.Token != token.Token {
		t.Errorf("expected token %v, got %v", token.Token, retrievedToken.Token)
	}

	if retrievedToken.UserID != userID {
		t.Errorf("expected user ID %v, got %v", userID, retrievedToken.UserID)
	}

	if retrievedToken.Used {
		t.Error("expected token to not be used")
	}
}

func TestPasswordResetService_GetToken_NotFound(t *testing.T) {
	redisClient := setupPasswordResetTestRedis(t)
	defer testutil.CleanupRedis(t)

	service := NewPasswordResetService(redisClient)

	_, err := service.GetToken(context.Background(), "nonexistent-token")
	if err != ErrPasswordResetTokenNotFound {
		t.Errorf("expected ErrPasswordResetTokenNotFound, got %v", err)
	}
}

func TestPasswordResetService_MarkTokenAsUsed(t *testing.T) {
	redisClient := setupPasswordResetTestRedis(t)
	defer testutil.CleanupRedis(t)

	service := NewPasswordResetService(redisClient)
	userID := uuid.New()

	token, err := service.GenerateToken(context.Background(), userID)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	if err := service.MarkTokenAsUsed(context.Background(), token.Token); err != nil {
		t.Fatalf("failed to mark token as used: %v", err)
	}

	retrievedToken, err := service.GetToken(context.Background(), token.Token)
	if err != nil {
		t.Fatalf("failed to get token: %v", err)
	}

	if !retrievedToken.Used {
		t.Error("expected token to be marked as used")
	}

	err = service.MarkTokenAsUsed(context.Background(), token.Token)
	if err != ErrPasswordResetTokenAlreadyUsed {
		t.Errorf("expected ErrPasswordResetTokenAlreadyUsed, got %v", err)
	}
}

func TestPasswordResetService_DeleteToken(t *testing.T) {
	redisClient := setupPasswordResetTestRedis(t)
	defer testutil.CleanupRedis(t)

	service := NewPasswordResetService(redisClient)
	userID := uuid.New()

	token, err := service.GenerateToken(context.Background(), userID)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	if err := service.DeleteToken(context.Background(), token.Token); err != nil {
		t.Fatalf("failed to delete token: %v", err)
	}

	_, err = service.GetToken(context.Background(), token.Token)
	if err != ErrPasswordResetTokenNotFound {
		t.Errorf("expected ErrPasswordResetTokenNotFound after deletion, got %v", err)
	}
}
