package services

import (
	"context"
	"encoding/json"
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
)

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
	if err := s.redis.Set(ctx, key, sessionJSON, SessionDuration).Err(); err != nil {
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
			return nil, fmt.Errorf("session not found or expired")
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
	if err := s.redis.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete session from Redis: %w", err)
	}
	return nil
}
