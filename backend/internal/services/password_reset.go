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
	"github.com/sanderginn/clubhouse/internal/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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
	ctx, span := otel.Tracer("clubhouse.password_reset").Start(ctx, "PasswordResetService.GenerateToken")
	span.SetAttributes(attribute.String("user_id", userID.String()))
	defer span.End()

	// Generate random token
	tokenBytes := make([]byte, PasswordResetTokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		recordSpanError(span, err)
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
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to marshal password reset token: %w", err)
	}

	// Store in Redis with expiration
	key := PasswordResetTokenPrefix + token
	if err := s.redis.Set(ctx, key, tokenJSON, PasswordResetTokenDuration).Err(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to store password reset token in Redis: %w", err)
	}

	return resetToken, nil
}

// GetToken retrieves a password reset token from Redis
func (s *PasswordResetService) GetToken(ctx context.Context, token string) (*PasswordResetToken, error) {
	ctx, span := otel.Tracer("clubhouse.password_reset").Start(ctx, "PasswordResetService.GetToken")
	span.SetAttributes(
		attribute.Bool("has_token", token != ""),
		attribute.Int("token_length", len(token)),
	)
	defer span.End()

	key := PasswordResetTokenPrefix + token
	tokenJSON, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			observability.RecordCacheMiss(ctx, "password_reset")
			recordSpanError(span, ErrPasswordResetTokenNotFound)
			return nil, ErrPasswordResetTokenNotFound
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to get password reset token from Redis: %w", err)
	}
	observability.RecordCacheHit(ctx, "password_reset")

	var resetToken PasswordResetToken
	if err := json.Unmarshal([]byte(tokenJSON), &resetToken); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to unmarshal password reset token: %w", err)
	}

	return &resetToken, nil
}

// MarkTokenAsUsed marks a password reset token as used (single-use enforcement)
// Uses Lua script for atomic check-and-set to prevent race conditions
func (s *PasswordResetService) MarkTokenAsUsed(ctx context.Context, token string) error {
	ctx, span := otel.Tracer("clubhouse.password_reset").Start(ctx, "PasswordResetService.MarkTokenAsUsed")
	span.SetAttributes(
		attribute.Bool("has_token", token != ""),
		attribute.Int("token_length", len(token)),
	)
	defer span.End()

	_, err := s.ClaimToken(ctx, token)
	if err != nil {
		recordSpanError(span, err)
	}
	return err
}

// ClaimToken atomically marks a password reset token as used and returns the token data.
// This prevents race conditions where concurrent requests could both read Used=false.
// Uses Lua script for atomic check-and-set.
func (s *PasswordResetService) ClaimToken(ctx context.Context, token string) (*PasswordResetToken, error) {
	ctx, span := otel.Tracer("clubhouse.password_reset").Start(ctx, "PasswordResetService.ClaimToken")
	span.SetAttributes(
		attribute.Bool("has_token", token != ""),
		attribute.Int("token_length", len(token)),
	)
	defer span.End()

	key := PasswordResetTokenPrefix + token

	// Lua script for atomic check-and-set
	// Returns: JSON string of token data if successfully claimed, empty string if already used, nil if not found
	script := `
		local key = KEYS[1]
		local tokenJSON = redis.call('GET', key)
		if not tokenJSON then
			return nil
		end

		local tokenData = cjson.decode(tokenJSON)
		if tokenData.used then
			return ""
		end

		tokenData.used = true
		local newTokenJSON = cjson.encode(tokenData)
		local ttl = redis.call('TTL', key)
		if ttl > 0 then
			redis.call('SETEX', key, ttl, newTokenJSON)
		else
			redis.call('SET', key, newTokenJSON)
		end
		return tokenJSON
	`

	result, err := s.redis.Eval(ctx, script, []string{key}).Result()
	if err != nil {
		if err == redis.Nil {
			observability.RecordCacheMiss(ctx, "password_reset")
			recordSpanError(span, ErrPasswordResetTokenNotFound)
			return nil, ErrPasswordResetTokenNotFound
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to execute atomic claim-token script: %w", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		observability.RecordCacheMiss(ctx, "password_reset")
		recordSpanError(span, ErrPasswordResetTokenNotFound)
		return nil, ErrPasswordResetTokenNotFound
	}

	if resultStr == "" {
		observability.RecordCacheHit(ctx, "password_reset")
		recordSpanError(span, ErrPasswordResetTokenAlreadyUsed)
		return nil, ErrPasswordResetTokenAlreadyUsed
	}
	observability.RecordCacheHit(ctx, "password_reset")

	var resetToken PasswordResetToken
	if err := json.Unmarshal([]byte(resultStr), &resetToken); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to unmarshal password reset token: %w", err)
	}

	return &resetToken, nil
}

// DeleteToken removes a password reset token from Redis
func (s *PasswordResetService) DeleteToken(ctx context.Context, token string) error {
	ctx, span := otel.Tracer("clubhouse.password_reset").Start(ctx, "PasswordResetService.DeleteToken")
	span.SetAttributes(
		attribute.Bool("has_token", token != ""),
		attribute.Int("token_length", len(token)),
	)
	defer span.End()

	key := PasswordResetTokenPrefix + token
	if err := s.redis.Del(ctx, key).Err(); err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to delete password reset token from Redis: %w", err)
	}
	return nil
}
