package services

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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
	ctx, span := otel.Tracer("clubhouse.audit").Start(ctx, "AuditService.LogAuditWithMetadata")
	span.SetAttributes(
		attribute.String("action", action),
		attribute.Bool("has_admin_user_id", adminUserID != uuid.Nil),
		attribute.Bool("has_target_user_id", targetUserID != uuid.Nil),
		attribute.Bool("has_metadata", metadata != nil),
	)
	if adminUserID != uuid.Nil {
		span.SetAttributes(attribute.String("admin_user_id", adminUserID.String()))
	}
	if targetUserID != uuid.Nil {
		span.SetAttributes(attribute.String("target_user_id", targetUserID.String()))
	}
	defer span.End()

	if s == nil || s.exec == nil {
		err := fmt.Errorf("audit service is not configured")
		recordSpanError(span, err)
		return err
	}
	if action == "" {
		err := fmt.Errorf("audit action is required")
		recordSpanError(span, err)
		return err
	}

	var adminUserValue interface{}
	if adminUserID != uuid.Nil {
		adminUserValue = adminUserID
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
	_, err := s.exec.ExecContext(ctx, query, adminUserValue, action, relatedUserValue, targetUserValue, metadataValue)
	if err != nil {
		recordSpanError(span, err)
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
	ctx, span := otel.Tracer("clubhouse.audit").Start(ctx, "AuditService.LogModerationAudit")
	span.SetAttributes(
		attribute.String("action", action),
		attribute.String("admin_user_id", adminUserID.String()),
		attribute.Bool("has_target_user_id", targetUserID != uuid.Nil),
		attribute.Bool("has_post_id", relatedPostID != uuid.Nil),
		attribute.Bool("has_comment_id", relatedCommentID != uuid.Nil),
		attribute.Bool("has_metadata", metadata != nil),
	)
	if targetUserID != uuid.Nil {
		span.SetAttributes(attribute.String("target_user_id", targetUserID.String()))
	}
	if relatedPostID != uuid.Nil {
		span.SetAttributes(attribute.String("post_id", relatedPostID.String()))
	}
	if relatedCommentID != uuid.Nil {
		span.SetAttributes(attribute.String("comment_id", relatedCommentID.String()))
	}
	defer span.End()

	if s == nil || s.exec == nil {
		err := fmt.Errorf("audit service is not configured")
		recordSpanError(span, err)
		return err
	}
	if action == "" {
		err := fmt.Errorf("audit action is required")
		recordSpanError(span, err)
		return err
	}
	if adminUserID == uuid.Nil {
		err := fmt.Errorf("admin user id is required")
		recordSpanError(span, err)
		return err
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
		recordSpanError(span, err)
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	return nil
}

// LogAudit records an admin audit log without metadata.
func (s *AuditService) LogAudit(ctx context.Context, action string, adminUserID uuid.UUID, targetUserID uuid.UUID) error {
	ctx, span := otel.Tracer("clubhouse.audit").Start(ctx, "AuditService.LogAudit")
	span.SetAttributes(
		attribute.String("action", action),
		attribute.Bool("has_admin_user_id", adminUserID != uuid.Nil),
		attribute.Bool("has_target_user_id", targetUserID != uuid.Nil),
	)
	if adminUserID != uuid.Nil {
		span.SetAttributes(attribute.String("admin_user_id", adminUserID.String()))
	}
	if targetUserID != uuid.Nil {
		span.SetAttributes(attribute.String("target_user_id", targetUserID.String()))
	}
	defer span.End()

	if err := s.LogAuditWithMetadata(ctx, action, adminUserID, targetUserID, nil); err != nil {
		recordSpanError(span, err)
		return err
	}
	return nil
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
