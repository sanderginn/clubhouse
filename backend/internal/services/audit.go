package services

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
)

type auditExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

const auditExcerptLimit = 100

// AuditService handles audit log writes.
type AuditService struct {
	exec auditExecutor
}

// NewAuditService creates a new audit service.
func NewAuditService(execer auditExecutor) *AuditService {
	return &AuditService{exec: execer}
}

// LogAuditWithMetadata records an admin audit log with optional metadata.
func (s *AuditService) LogAuditWithMetadata(
	ctx context.Context,
	action string,
	adminUserID uuid.UUID,
	targetUserID uuid.UUID,
	metadata map[string]interface{},
) error {
	if s == nil || s.exec == nil {
		return fmt.Errorf("audit service is not configured")
	}
	if action == "" {
		return fmt.Errorf("audit action is required")
	}
	if adminUserID == uuid.Nil {
		return fmt.Errorf("admin user id is required")
	}

	var targetUserValue interface{}
	var relatedUserValue interface{}
	if targetUserID != uuid.Nil {
		targetUserValue = targetUserID
		relatedUserValue = targetUserID
	}

	var metadataValue interface{}
	if metadata != nil {
		metadataValue = models.JSONMap(metadata)
	}

	query := `
		INSERT INTO audit_logs (admin_user_id, action, related_user_id, target_user_id, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, now())
	`
	_, err := s.exec.ExecContext(ctx, query, adminUserID, action, relatedUserValue, targetUserValue, metadataValue)
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	return nil
}

// LogModerationAudit records an admin audit log with optional metadata and related entities.
func (s *AuditService) LogModerationAudit(
	ctx context.Context,
	action string,
	adminUserID uuid.UUID,
	targetUserID uuid.UUID,
	relatedPostID uuid.UUID,
	relatedCommentID uuid.UUID,
	metadata map[string]interface{},
) error {
	if s == nil || s.exec == nil {
		return fmt.Errorf("audit service is not configured")
	}
	if action == "" {
		return fmt.Errorf("audit action is required")
	}
	if adminUserID == uuid.Nil {
		return fmt.Errorf("admin user id is required")
	}

	var targetUserValue interface{}
	var relatedUserValue interface{}
	if targetUserID != uuid.Nil {
		targetUserValue = targetUserID
		relatedUserValue = targetUserID
	}

	var relatedPostValue interface{}
	if relatedPostID != uuid.Nil {
		relatedPostValue = relatedPostID
	}

	var relatedCommentValue interface{}
	if relatedCommentID != uuid.Nil {
		relatedCommentValue = relatedCommentID
	}

	var metadataValue interface{}
	if metadata != nil {
		metadataValue = models.JSONMap(metadata)
	}

	query := `
		INSERT INTO audit_logs (
			admin_user_id,
			action,
			related_post_id,
			related_comment_id,
			related_user_id,
			target_user_id,
			metadata,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now())
	`
	_, err := s.exec.ExecContext(
		ctx,
		query,
		adminUserID,
		action,
		relatedPostValue,
		relatedCommentValue,
		relatedUserValue,
		targetUserValue,
		metadataValue,
	)
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	return nil
}

// LogAudit records an admin audit log without metadata.
func (s *AuditService) LogAudit(ctx context.Context, action string, adminUserID uuid.UUID, targetUserID uuid.UUID) error {
	return s.LogAuditWithMetadata(ctx, action, adminUserID, targetUserID, nil)
}

func truncateAuditExcerpt(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) > auditExcerptLimit {
		return string(runes[:auditExcerptLimit])
	}
	return trimmed
}
