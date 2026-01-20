package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system
type User struct {
	ID                uuid.UUID  `json:"id"`
	Username          string     `json:"username"`
	Email             string     `json:"email"`
	PasswordHash      string     `json:"-"` // Never expose
	ProfilePictureURL *string    `json:"profile_picture_url,omitempty"`
	Bio               *string    `json:"bio,omitempty"`
	IsAdmin           bool       `json:"is_admin"`
	ApprovedAt        *time.Time `json:"approved_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         *time.Time `json:"updated_at,omitempty"`
	DeletedAt         *time.Time `json:"deleted_at,omitempty"`
}

// RegisterRequest represents the registration request body
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterResponse represents the registration response
type RegisterResponse struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Message  string    `json:"message"`
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	IsAdmin  bool      `json:"is_admin"`
	Message  string    `json:"message"`
}

// LogoutResponse represents the logout response
type LogoutResponse struct {
	Message string `json:"message"`
}

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// PendingUser represents a user pending admin approval
type PendingUser struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// ApproveUserResponse represents the response from approving a user
type ApproveUserResponse struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Message  string    `json:"message"`
}

// RejectUserResponse represents the response from rejecting a user
type RejectUserResponse struct {
	ID      uuid.UUID `json:"id"`
	Message string    `json:"message"`
}

// MeResponse represents the response from /auth/me endpoint
type MeResponse struct {
	ID                uuid.UUID `json:"id"`
	Username          string    `json:"username"`
	Email             string    `json:"email"`
	ProfilePictureUrl *string   `json:"profile_picture_url,omitempty"`
	Bio               *string   `json:"bio,omitempty"`
	IsAdmin           bool      `json:"is_admin"`
}

// UserStats represents user activity statistics
type UserStats struct {
	PostCount    int `json:"post_count"`
	CommentCount int `json:"comment_count"`
}

// UserProfileResponse represents the response from /users/{id} endpoint
type UserProfileResponse struct {
	ID                uuid.UUID `json:"id"`
	Username          string    `json:"username"`
	Bio               *string   `json:"bio,omitempty"`
	ProfilePictureUrl *string   `json:"profile_picture_url,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	Stats             UserStats `json:"stats"`
}

// UpdateUserRequest represents the request to update user profile
type UpdateUserRequest struct {
	Bio               *string `json:"bio,omitempty"`
	ProfilePictureUrl *string `json:"profile_picture_url,omitempty"`
}

// UpdateUserResponse represents the response from updating user profile
type UpdateUserResponse struct {
	ID                uuid.UUID `json:"id"`
	Username          string    `json:"username"`
	Email             string    `json:"email"`
	ProfilePictureUrl *string   `json:"profile_picture_url,omitempty"`
	Bio               *string   `json:"bio,omitempty"`
	IsAdmin           bool      `json:"is_admin"`
}

// SectionSubscription represents an opt-out entry for a section.
type SectionSubscription struct {
	SectionID  uuid.UUID `json:"section_id"`
	OptedOutAt time.Time `json:"opted_out_at"`
}

// GetSectionSubscriptionsResponse represents the response from listing section opt-outs.
type GetSectionSubscriptionsResponse struct {
	SectionSubscriptions []SectionSubscription `json:"section_subscriptions"`
}

// UpdateSectionSubscriptionRequest represents a request to opt in/out of section notifications.
type UpdateSectionSubscriptionRequest struct {
	OptedOut *bool `json:"opted_out"`
}

// UpdateSectionSubscriptionResponse represents the response from updating section opt-out status.
type UpdateSectionSubscriptionResponse struct {
	SectionID  uuid.UUID  `json:"section_id"`
	OptedOut   bool       `json:"opted_out"`
	OptedOutAt *time.Time `json:"opted_out_at,omitempty"`
}
