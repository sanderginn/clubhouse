package services

import (
	"context"
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
	// SessionDuration is the duration a session is valid (30 days)
	SessionDuration = 30 * 24 * time.Hour
	// SessionKeyPrefix is the Redis key prefix for sessions
	SessionKeyPrefix = "session:"
	// UserSessionSetPrefix is the Redis key prefix for user session sets
	UserSessionSetPrefix = "user_sessions:"
)

// ErrSessionNotFound is returned when a session cannot be found in Redis.
var ErrSessionNotFound = errors.New("session not found or expired")

// Session represents a user session stored in Redis
type Session struct {
	ID        string    `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SessionService handles session-related operations
type SessionService struct {
	redis *redis.Client
}

// NewSessionService creates a new session service
func NewSessionService(redis *redis.Client) *SessionService {
	return &SessionService{redis: redis}
}

// CreateSession creates a new session for a user
func (s *SessionService) CreateSession(ctx context.Context, userID uuid.UUID, username string, isAdmin bool) (*Session, error) {
	ctx, span := otel.Tracer("clubhouse.sessions").Start(ctx, "SessionService.CreateSession")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.Bool("is_admin", isAdmin),
		attribute.Bool("has_username", username != ""),
	)
	defer span.End()

	sessionID := uuid.New().String()
	now := time.Now().UTC()
	expiresAt := now.Add(SessionDuration)

	session := &Session{
		ID:        sessionID,
		UserID:    userID,
		Username:  username,
		IsAdmin:   isAdmin,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}

	// Marshal session to JSON
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to marshal session: %w", err)
	}

	// Store in Redis with expiration
	key := SessionKeyPrefix + sessionID
	userKey := UserSessionSetPrefix + userID.String()
	pipe := s.redis.TxPipeline()
	pipe.Set(ctx, key, sessionJSON, SessionDuration)
	pipe.SAdd(ctx, userKey, sessionID)
	pipe.Expire(ctx, userKey, SessionDuration)
	if _, err := pipe.Exec(ctx); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to store session in Redis: %w", err)
	}

	observability.RecordAuthSessionCreated(ctx)

	return session, nil
}

// GetSession retrieves a session from Redis
func (s *SessionService) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	ctx, span := otel.Tracer("clubhouse.sessions").Start(ctx, "SessionService.GetSession")
	span.SetAttributes(attribute.String("session_id", sessionID))
	defer span.End()

	key := SessionKeyPrefix + sessionID
	sessionJSON, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			observability.RecordCacheMiss(ctx, "session")
			observability.RecordAuthSessionExpired(ctx, "timeout", 1)
			observability.RecordAuthFailure(ctx, "expired_session")
			recordSpanError(span, ErrSessionNotFound)
			return nil, ErrSessionNotFound
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to get session from Redis: %w", err)
	}
	observability.RecordCacheHit(ctx, "session")

	var session Session
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// DeleteSession removes a session from Redis
func (s *SessionService) DeleteSession(ctx context.Context, sessionID string) error {
	ctx, span := otel.Tracer("clubhouse.sessions").Start(ctx, "SessionService.DeleteSession")
	span.SetAttributes(attribute.String("session_id", sessionID))
	defer span.End()

	key := SessionKeyPrefix + sessionID
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}
		recordSpanError(span, err)
		return fmt.Errorf("failed to get session for deletion: %w", err)
	}

	userKey := UserSessionSetPrefix + session.UserID.String()
	pipe := s.redis.TxPipeline()
	pipe.SRem(ctx, userKey, sessionID)
	pipe.Del(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to delete session from Redis: %w", err)
	}

	return nil
}

// DeleteAllSessionsForUser removes all sessions for a user from Redis.
func (s *SessionService) DeleteAllSessionsForUser(ctx context.Context, userID uuid.UUID) (int, error) {
	ctx, span := otel.Tracer("clubhouse.sessions").Start(ctx, "SessionService.DeleteAllSessionsForUser")
	span.SetAttributes(attribute.String("user_id", userID.String()))
	defer span.End()

	userKey := UserSessionSetPrefix + userID.String()
	sessionIDs, err := s.redis.SMembers(ctx, userKey).Result()
	if err != nil {
		recordSpanError(span, err)
		return 0, fmt.Errorf("failed to list user sessions: %w", err)
	}

	pipe := s.redis.TxPipeline()
	for _, sessionID := range sessionIDs {
		pipe.Del(ctx, SessionKeyPrefix+sessionID)
	}
	pipe.Del(ctx, userKey)
	if _, err := pipe.Exec(ctx); err != nil {
		recordSpanError(span, err)
		return 0, fmt.Errorf("failed to delete user sessions: %w", err)
	}

	return len(sessionIDs), nil
}

// UpdateUserAdminStatus updates cached session admin status for all of a user's sessions.
func (s *SessionService) UpdateUserAdminStatus(ctx context.Context, userID uuid.UUID, isAdmin bool) error {
	ctx, span := otel.Tracer("clubhouse.sessions").Start(ctx, "SessionService.UpdateUserAdminStatus")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.Bool("is_admin", isAdmin),
	)
	defer span.End()

	if s == nil || s.redis == nil {
		err := fmt.Errorf("session service is not configured")
		recordSpanError(span, err)
		return err
	}

	userKey := UserSessionSetPrefix + userID.String()
	sessionIDs, err := s.redis.SMembers(ctx, userKey).Result()
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to list user sessions: %w", err)
	}
	if len(sessionIDs) == 0 {
		return nil
	}

	pipe := s.redis.TxPipeline()
	for _, sessionID := range sessionIDs {
		key := SessionKeyPrefix + sessionID
		sessionJSON, err := s.redis.Get(ctx, key).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			recordSpanError(span, err)
			return fmt.Errorf("failed to get session: %w", err)
		}

		var session Session
		if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
			recordSpanError(span, err)
			return fmt.Errorf("failed to unmarshal session: %w", err)
		}

		session.IsAdmin = isAdmin
		updatedJSON, err := json.Marshal(&session)
		if err != nil {
			recordSpanError(span, err)
			return fmt.Errorf("failed to marshal session: %w", err)
		}

		ttl, err := s.redis.TTL(ctx, key).Result()
		if err != nil {
			recordSpanError(span, err)
			return fmt.Errorf("failed to get session ttl: %w", err)
		}
		if ttl <= 0 {
			ttl = SessionDuration
		}
		pipe.Set(ctx, key, updatedJSON, ttl)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to update user sessions: %w", err)
	}

	return nil
}
