package models

import (
	"time"

	"github.com/google/uuid"
)

// AuthEvent represents an authentication-related event for security auditing.
type AuthEvent struct {
	ID         uuid.UUID  `json:"id"`
	UserID     *uuid.UUID `json:"user_id,omitempty"`
	Username   *string    `json:"username,omitempty"`
	Identifier string     `json:"identifier,omitempty"`
	EventType  string     `json:"event_type"`
	IPAddress  string     `json:"ip_address,omitempty"`
	UserAgent  string     `json:"user_agent,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// AuthEventCreate represents the fields required to log an auth event.
type AuthEventCreate struct {
	UserID     *uuid.UUID
	Identifier string
	EventType  string
	IPAddress  string
	UserAgent  string
}

// AuthEventLogsResponse represents the response for listing auth events.
type AuthEventLogsResponse struct {
	Events     []*AuthEvent `json:"events"`
	HasMore    bool         `json:"has_more"`
	NextCursor *string      `json:"next_cursor,omitempty"`
}
