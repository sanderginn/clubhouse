package middleware

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/services"
)

// GetUserFromContext extracts the user session from the request context
func GetUserFromContext(ctx context.Context) (*services.Session, error) {
	user := ctx.Value(UserContextKey)
	if user == nil {
		return nil, fmt.Errorf("user not found in context")
	}

	session, ok := user.(*services.Session)
	if !ok {
		return nil, fmt.Errorf("invalid user context type")
	}

	return session, nil
}

// GetSessionIDFromContext extracts the session ID from the request context
func GetSessionIDFromContext(ctx context.Context) (string, error) {
	sessionID := ctx.Value(SessionIDContextKey)
	if sessionID == nil {
		return "", fmt.Errorf("session id not found in context")
	}

	id, ok := sessionID.(string)
	if !ok {
		return "", fmt.Errorf("invalid session id context type")
	}

	return id, nil
}

// GetUserIDFromContext extracts the user ID from the request context
func GetUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	session, err := GetUserFromContext(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return session.UserID, nil
}

// GetUsernameFromContext extracts the username from the request context
func GetUsernameFromContext(ctx context.Context) (string, error) {
	session, err := GetUserFromContext(ctx)
	if err != nil {
		return "", err
	}
	return session.Username, nil
}

// GetIsAdminFromContext extracts the admin flag from the request context
func GetIsAdminFromContext(ctx context.Context) (bool, error) {
	session, err := GetUserFromContext(ctx)
	if err != nil {
		return false, err
	}
	return session.IsAdmin, nil
}

// GetSectionIDFromContext extracts the current section ID from the request context
func GetSectionIDFromContext(ctx context.Context) (uuid.UUID, error) {
	sectionID := ctx.Value(SectionIDContextKey)
	if sectionID == nil {
		return uuid.Nil, fmt.Errorf("section id not found in context")
	}
	parsedID, ok := sectionID.(uuid.UUID)
	if !ok {
		return uuid.Nil, fmt.Errorf("invalid section id context type")
	}
	return parsedID, nil
}
