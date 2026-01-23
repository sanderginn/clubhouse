package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// PasswordResetTokenDuration is the duration a password reset token is valid (1 hour)
	PasswordResetTokenDuration = 1 * time.Hour
	// PasswordResetTokenPrefix is the Redis key prefix for password reset tokens
	PasswordResetTokenPrefix = "password_reset:"
	// PasswordResetTokenLength is the number of random bytes to generate (will be base64 encoded)
	PasswordResetTokenLength = 32
)

// ErrPasswordResetTokenNotFound is returned when a password reset token cannot be found in Redis.
var ErrPasswordResetTokenNotFound = errors.New("password reset token not found or expired")

// ErrPasswordResetTokenAlreadyUsed is returned when a password reset token has already been used.
var ErrPasswordResetTokenAlreadyUsed = errors.New("password reset token has already been used")

// PasswordResetToken represents a password reset token stored in Redis
type PasswordResetToken struct {
	Token     string    `json:"token"`
	UserID    uuid.UUID `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
}

// PasswordResetService handles password reset token operations
type PasswordResetService struct {
	redis *redis.Client
}

// NewPasswordResetService creates a new password reset service
func NewPasswordResetService(redis *redis.Client) *PasswordResetService {
	return &PasswordResetService{redis: redis}
}

// GenerateToken creates a new password reset token for a user
func (s *PasswordResetService) GenerateToken(ctx context.Context, userID uuid.UUID) (*PasswordResetToken, error) {
	// Generate random token
	tokenBytes := make([]byte, PasswordResetTokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	now := time.Now().UTC()
	expiresAt := now.Add(PasswordResetTokenDuration)

	resetToken := &PasswordResetToken{
		Token:     token,
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: expiresAt,
		Used:      false,
	}

	// Marshal token to JSON
	tokenJSON, err := json.Marshal(resetToken)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal password reset token: %w", err)
	}

	// Store in Redis with expiration
	key := PasswordResetTokenPrefix + token
	if err := s.redis.Set(ctx, key, tokenJSON, PasswordResetTokenDuration).Err(); err != nil {
		return nil, fmt.Errorf("failed to store password reset token in Redis: %w", err)
	}

	return resetToken, nil
}

// GetToken retrieves a password reset token from Redis
func (s *PasswordResetService) GetToken(ctx context.Context, token string) (*PasswordResetToken, error) {
	key := PasswordResetTokenPrefix + token
	tokenJSON, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrPasswordResetTokenNotFound
		}
		return nil, fmt.Errorf("failed to get password reset token from Redis: %w", err)
	}

	var resetToken PasswordResetToken
	if err := json.Unmarshal([]byte(tokenJSON), &resetToken); err != nil {
		return nil, fmt.Errorf("failed to unmarshal password reset token: %w", err)
	}

	return &resetToken, nil
}

// MarkTokenAsUsed marks a password reset token as used (single-use enforcement)
func (s *PasswordResetService) MarkTokenAsUsed(ctx context.Context, token string) error {
	resetToken, err := s.GetToken(ctx, token)
	if err != nil {
		return err
	}

	if resetToken.Used {
		return ErrPasswordResetTokenAlreadyUsed
	}

	// Mark as used
	resetToken.Used = true
	tokenJSON, err := json.Marshal(resetToken)
	if err != nil {
		return fmt.Errorf("failed to marshal password reset token: %w", err)
	}

	// Update in Redis (keep original TTL)
	key := PasswordResetTokenPrefix + token
	ttl := time.Until(resetToken.ExpiresAt)
	if ttl <= 0 {
		// Token has already expired
		return ErrPasswordResetTokenNotFound
	}

	if err := s.redis.Set(ctx, key, tokenJSON, ttl).Err(); err != nil {
		return fmt.Errorf("failed to update password reset token in Redis: %w", err)
	}

	return nil
}

// DeleteToken removes a password reset token from Redis
func (s *PasswordResetService) DeleteToken(ctx context.Context, token string) error {
	key := PasswordResetTokenPrefix + token
	if err := s.redis.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete password reset token from Redis: %w", err)
	}
	return nil
}
