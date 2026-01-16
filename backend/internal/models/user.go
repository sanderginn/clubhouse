package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system
type User struct {
	ID                 uuid.UUID  `json:"id"`
	Username           string     `json:"username"`
	Email              string     `json:"email"`
	PasswordHash       string     `json:"-"` // Never expose
	ProfilePictureURL  *string    `json:"profile_picture_url,omitempty"`
	Bio                *string    `json:"bio,omitempty"`
	IsAdmin            bool       `json:"is_admin"`
	ApprovedAt         *time.Time `json:"approved_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          *time.Time `json:"updated_at,omitempty"`
	DeletedAt          *time.Time `json:"deleted_at,omitempty"`
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

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}
