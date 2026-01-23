package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
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
		return nil, fmt.Errorf("failed to store session in Redis: %w", err)
	}

	return session, nil
}

// GetSession retrieves a session from Redis
func (s *SessionService) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	key := SessionKeyPrefix + sessionID
	sessionJSON, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get session from Redis: %w", err)
	}

	var session Session
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// DeleteSession removes a session from Redis
func (s *SessionService) DeleteSession(ctx context.Context, sessionID string) error {
	key := SessionKeyPrefix + sessionID
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}
		return fmt.Errorf("failed to get session for deletion: %w", err)
	}

	userKey := UserSessionSetPrefix + session.UserID.String()
	pipe := s.redis.TxPipeline()
	pipe.SRem(ctx, userKey, sessionID)
	pipe.Del(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete session from Redis: %w", err)
	}

	return nil
}

// DeleteAllSessionsForUser removes all sessions for a user from Redis.
func (s *SessionService) DeleteAllSessionsForUser(ctx context.Context, userID uuid.UUID) error {
	userKey := UserSessionSetPrefix + userID.String()
	sessionIDs, err := s.redis.SMembers(ctx, userKey).Result()
	if err != nil {
		return fmt.Errorf("failed to list user sessions: %w", err)
	}

	pipe := s.redis.TxPipeline()
	for _, sessionID := range sessionIDs {
		pipe.Del(ctx, SessionKeyPrefix+sessionID)
	}
	pipe.Del(ctx, userKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	return nil
}
