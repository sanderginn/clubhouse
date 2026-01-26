package services

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sanderginn/clubhouse/internal/models"
)

// AuthEventService handles auth event logging.
type AuthEventService struct {
	db *sql.DB
}

// NewAuthEventService creates a new auth event service.
func NewAuthEventService(db *sql.DB) *AuthEventService {
	return &AuthEventService{db: db}
}

// LogEvent records an authentication-related event for auditing.
func (s *AuthEventService) LogEvent(ctx context.Context, event *models.AuthEventCreate) error {
	if event == nil {
		return fmt.Errorf("auth event is required")
	}
	if event.EventType == "" {
		return fmt.Errorf("auth event type is required")
	}

	query := `
		INSERT INTO auth_events (user_id, identifier, event_type, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, now())
	`
	_, err := s.db.ExecContext(ctx, query, event.UserID, event.Identifier, event.EventType, event.IPAddress, event.UserAgent)
	if err != nil {
		return fmt.Errorf("failed to insert auth event: %w", err)
	}

	return nil
}
