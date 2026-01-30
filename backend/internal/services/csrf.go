package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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
	// CSRFTokenDuration is the duration a CSRF token is valid (1 hour)
	CSRFTokenDuration = 1 * time.Hour
	// CSRFKeyPrefix is the Redis key prefix for CSRF tokens
	CSRFKeyPrefix = "csrf:"
	// CSRFTokenLength is the length of the CSRF token in bytes (32 bytes = 256 bits)
	CSRFTokenLength = 32
)

// ErrCSRFTokenNotFound is returned when a CSRF token cannot be found in Redis.
var ErrCSRFTokenNotFound = errors.New("csrf token not found or expired")

// CSRFService handles CSRF token operations
type CSRFService struct {
	redis *redis.Client
}

// NewCSRFService creates a new CSRF service
func NewCSRFService(redis *redis.Client) *CSRFService {
	return &CSRFService{redis: redis}
}

// GenerateToken generates a new CSRF token for a user session
func (s *CSRFService) GenerateToken(ctx context.Context, sessionID string, userID uuid.UUID) (string, error) {
	ctx, span := otel.Tracer("clubhouse.csrf").Start(ctx, "CSRFService.GenerateToken")
	span.SetAttributes(
		attribute.String("session_id", sessionID),
		attribute.String("user_id", userID.String()),
	)
	defer span.End()

	// Generate cryptographically secure random token
	tokenBytes := make([]byte, CSRFTokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		recordSpanError(span, err)
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	// Encode as base64 for safe transport
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	// Store in Redis with session ID and user ID as value
	key := CSRFKeyPrefix + token
	value := fmt.Sprintf("%s:%s", sessionID, userID.String())

	if err := s.redis.Set(ctx, key, value, CSRFTokenDuration).Err(); err != nil {
		recordSpanError(span, err)
		return "", fmt.Errorf("failed to store CSRF token in Redis: %w", err)
	}

	return token, nil
}

// ValidateToken validates a CSRF token and returns the associated session ID and user ID
func (s *CSRFService) ValidateToken(ctx context.Context, token string, sessionID string, userID uuid.UUID) error {
	ctx, span := otel.Tracer("clubhouse.csrf").Start(ctx, "CSRFService.ValidateToken")
	span.SetAttributes(
		attribute.Bool("has_token", token != ""),
		attribute.Int("token_length", len(token)),
		attribute.String("session_id", sessionID),
		attribute.String("user_id", userID.String()),
	)
	defer span.End()

	if token == "" {
		missingErr := errors.New("csrf token is required")
		recordSpanError(span, missingErr)
		return missingErr
	}

	key := CSRFKeyPrefix + token
	value, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			observability.RecordCacheMiss(ctx, "csrf")
			recordSpanError(span, ErrCSRFTokenNotFound)
			return ErrCSRFTokenNotFound
		}
		recordSpanError(span, err)
		return fmt.Errorf("failed to get CSRF token from Redis: %w", err)
	}
	observability.RecordCacheHit(ctx, "csrf")

	// Verify the token is for this session and user
	expectedValue := fmt.Sprintf("%s:%s", sessionID, userID.String())
	if value != expectedValue {
		mismatchErr := errors.New("csrf token does not match session")
		recordSpanError(span, mismatchErr)
		return mismatchErr
	}

	return nil
}

// DeleteToken removes a CSRF token from Redis
func (s *CSRFService) DeleteToken(ctx context.Context, token string) error {
	ctx, span := otel.Tracer("clubhouse.csrf").Start(ctx, "CSRFService.DeleteToken")
	span.SetAttributes(
		attribute.Bool("has_token", token != ""),
		attribute.Int("token_length", len(token)),
	)
	defer span.End()

	key := CSRFKeyPrefix + token
	if err := s.redis.Del(ctx, key).Err(); err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to delete CSRF token from Redis: %w", err)
	}
	return nil
}
