package services

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sanderginn/clubhouse/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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
	ctx, span := otel.Tracer("clubhouse.auth_events").Start(ctx, "AuthEventService.LogEvent")
	defer span.End()

	if s == nil || s.db == nil {
		err := fmt.Errorf("auth event service is not configured")
		recordSpanError(span, err)
		return err
	}

	if event == nil {
		err := fmt.Errorf("auth event is required")
		recordSpanError(span, err)
		return err
	}
	if event.EventType == "" {
		err := fmt.Errorf("auth event type is required")
		recordSpanError(span, err)
		return err
	}

	span.SetAttributes(
		attribute.Bool("has_user_id", event.UserID != nil),
		attribute.String("event_type", event.EventType),
	)
	if event.UserID != nil {
		span.SetAttributes(attribute.String("user_id", event.UserID.String()))
	}

	query := `
		INSERT INTO auth_events (user_id, identifier, event_type, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, now())
	`
	_, err := s.db.ExecContext(ctx, query, event.UserID, event.Identifier, event.EventType, event.IPAddress, event.UserAgent)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to insert auth event: %w", err)
	}

	return nil
}
