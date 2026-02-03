package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const highlightReactionEmoji = "❤️"

type HighlightReactionService struct {
	db *sql.DB
}

func NewHighlightReactionService(db *sql.DB) *HighlightReactionService {
	return &HighlightReactionService{db: db}
}

func (s *HighlightReactionService) AddReaction(ctx context.Context, postID uuid.UUID, highlightID string, userID uuid.UUID) (*models.HighlightReactionResponse, error) {
	ctx, span := otel.Tracer("clubhouse.highlight_reactions").Start(ctx, "HighlightReactionService.AddReaction")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.String("user_id", userID.String()),
	)
	defer span.End()

	linkID, highlight, err := s.resolveHighlight(ctx, postID, highlightID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO highlight_reactions (id, user_id, link_id, highlight_id, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, now())
		ON CONFLICT (user_id, highlight_id) DO NOTHING
	`, userID, linkID, highlightID)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to create highlight reaction: %w", err)
	}

	state, err := s.getReactionState(ctx, highlightID, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if err := s.logHighlightReactionAudit(ctx, "add_highlight_reaction", userID, map[string]interface{}{
		"post_id":      postID.String(),
		"link_id":      linkID.String(),
		"highlight_id": highlightID,
		"timestamp":    highlight.Timestamp,
		"label":        highlight.Label,
		"emoji":        highlightReactionEmoji,
	}); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return state, nil
}

func (s *HighlightReactionService) RemoveReaction(ctx context.Context, postID uuid.UUID, highlightID string, userID uuid.UUID) (*models.HighlightReactionResponse, error) {
	ctx, span := otel.Tracer("clubhouse.highlight_reactions").Start(ctx, "HighlightReactionService.RemoveReaction")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.String("user_id", userID.String()),
	)
	defer span.End()

	linkID, highlight, err := s.resolveHighlight(ctx, postID, highlightID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	result, err := s.db.ExecContext(ctx, `
		DELETE FROM highlight_reactions
		WHERE user_id = $1 AND highlight_id = $2
	`, userID, highlightID)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to remove highlight reaction: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to confirm highlight reaction removal: %w", err)
	}
	if rowsAffected == 0 {
		notFoundErr := errors.New("reaction not found")
		recordSpanError(span, notFoundErr)
		return nil, notFoundErr
	}

	state, err := s.getReactionState(ctx, highlightID, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if err := s.logHighlightReactionAudit(ctx, "remove_highlight_reaction", userID, map[string]interface{}{
		"post_id":      postID.String(),
		"link_id":      linkID.String(),
		"highlight_id": highlightID,
		"timestamp":    highlight.Timestamp,
		"label":        highlight.Label,
		"emoji":        highlightReactionEmoji,
	}); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return state, nil
}

func (s *HighlightReactionService) getReactionState(ctx context.Context, highlightID string, userID uuid.UUID) (*models.HighlightReactionResponse, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM highlight_reactions
		WHERE highlight_id = $1
	`, highlightID).Scan(&count); err != nil {
		return nil, fmt.Errorf("failed to fetch highlight reaction count: %w", err)
	}

	viewerReacted := false
	if userID != uuid.Nil {
		var exists bool
		if err := s.db.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1
				FROM highlight_reactions
				WHERE highlight_id = $1 AND user_id = $2
			)
		`, highlightID, userID).Scan(&exists); err != nil {
			return nil, fmt.Errorf("failed to fetch viewer highlight reaction: %w", err)
		}
		viewerReacted = exists
	}

	return &models.HighlightReactionResponse{
		HighlightID:   highlightID,
		HeartCount:    count,
		ViewerReacted: viewerReacted,
	}, nil
}

func (s *HighlightReactionService) resolveHighlight(ctx context.Context, postID uuid.UUID, highlightID string) (uuid.UUID, models.Highlight, error) {
	linkID, highlight, err := models.DecodeHighlightID(highlightID)
	if err != nil {
		return uuid.UUID{}, models.Highlight{}, errors.New("invalid highlight id")
	}

	var metadataJSON sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT metadata
		FROM links
		WHERE id = $1 AND post_id = $2
	`, linkID, postID).Scan(&metadataJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.UUID{}, models.Highlight{}, errors.New("highlight not found")
		}
		return uuid.UUID{}, models.Highlight{}, fmt.Errorf("failed to fetch highlight: %w", err)
	}
	if !metadataJSON.Valid {
		return uuid.UUID{}, models.Highlight{}, errors.New("highlight not found")
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err != nil {
		return uuid.UUID{}, models.Highlight{}, fmt.Errorf("failed to parse highlight metadata: %w", err)
	}

	raw, ok := metadata["highlights"]
	if !ok {
		return uuid.UUID{}, models.Highlight{}, errors.New("highlight not found")
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return uuid.UUID{}, models.Highlight{}, fmt.Errorf("failed to parse highlight metadata: %w", err)
	}
	var highlights []models.Highlight
	if err := json.Unmarshal(encoded, &highlights); err != nil {
		return uuid.UUID{}, models.Highlight{}, fmt.Errorf("failed to parse highlight metadata: %w", err)
	}

	for _, candidate := range highlights {
		if candidate.Timestamp == highlight.Timestamp && candidate.Label == highlight.Label {
			return linkID, candidate, nil
		}
	}

	return uuid.UUID{}, models.Highlight{}, errors.New("highlight not found")
}

func (s *HighlightReactionService) logHighlightReactionAudit(ctx context.Context, action string, userID uuid.UUID, metadata map[string]interface{}) error {
	auditService := NewAuditService(s.db)
	if err := auditService.LogAuditWithMetadata(ctx, action, uuid.Nil, userID, metadata); err != nil {
		observability.LogError(ctx, observability.ErrorLog{
			Message:    "failed to create highlight reaction audit log",
			Code:       "HIGHLIGHT_REACTION_AUDIT_FAILED",
			StatusCode: 500,
			Err:        err,
		})
		return fmt.Errorf("failed to create highlight reaction audit log: %w", err)
	}
	return nil
}
