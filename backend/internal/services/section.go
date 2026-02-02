package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type SectionService struct {
	db *sql.DB
}

func NewSectionService(db *sql.DB) *SectionService {
	return &SectionService{db: db}
}

func (s *SectionService) ListSections(ctx context.Context) ([]models.Section, error) {
	ctx, span := otel.Tracer("clubhouse.sections").Start(ctx, "SectionService.ListSections")
	defer span.End()

	query := `
		SELECT id, name, type
		FROM sections
		ORDER BY CASE type
			WHEN 'general' THEN 1
			WHEN 'music' THEN 2
			WHEN 'movie' THEN 3
			WHEN 'series' THEN 4
			WHEN 'recipe' THEN 5
			WHEN 'book' THEN 6
			WHEN 'event' THEN 7
			ELSE 100
		END,
		name ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	defer rows.Close()

	var sections []models.Section
	for rows.Next() {
		var section models.Section
		if err := rows.Scan(&section.ID, &section.Name, &section.Type); err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		sections = append(sections, section)
	}

	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	if sections == nil {
		sections = []models.Section{}
	}

	return sections, nil
}

func (s *SectionService) GetSectionByID(ctx context.Context, id uuid.UUID) (*models.Section, error) {
	ctx, span := otel.Tracer("clubhouse.sections").Start(ctx, "SectionService.GetSectionByID")
	span.SetAttributes(attribute.String("section_id", id.String()))
	defer span.End()

	query := `SELECT id, name, type FROM sections WHERE id = $1`

	var section models.Section
	err := s.db.QueryRowContext(ctx, query, id).Scan(&section.ID, &section.Name, &section.Type)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := errors.New("section not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, err
	}

	return &section, nil
}

func (s *SectionService) GetSectionLinks(ctx context.Context, sectionID uuid.UUID, cursor *string, limit int) (*models.SectionLinksResponse, error) {
	ctx, span := otel.Tracer("clubhouse.sections").Start(ctx, "SectionService.GetSectionLinks")
	span.SetAttributes(
		attribute.String("section_id", sectionID.String()),
		attribute.Int("limit", limit),
		attribute.Bool("has_cursor", cursor != nil && *cursor != ""),
	)
	defer span.End()

	if limit <= 0 {
		limit = 15
	}
	if limit > 50 {
		limit = 50
	}

	var exists bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sections WHERE id = $1)", sectionID).Scan(&exists)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	if !exists {
		notFoundErr := errors.New("section not found")
		recordSpanError(span, notFoundErr)
		return nil, notFoundErr
	}

	query := `
		SELECT
			l.id, l.url, l.metadata, l.post_id,
			p.user_id, u.username, l.created_at
		FROM links l
		JOIN posts p ON l.post_id = p.id
		JOIN users u ON p.user_id = u.id
		WHERE p.section_id = $1 AND p.deleted_at IS NULL
	`

	args := []interface{}{sectionID}
	argIndex := 2

	if cursor != nil && *cursor != "" {
		parsedCursor, err := time.Parse(time.RFC3339Nano, *cursor)
		if err != nil {
			invalidErr := errors.New("invalid cursor")
			recordSpanError(span, invalidErr)
			return nil, invalidErr
		}

		query += " AND l.created_at < $" + fmt.Sprintf("%d", argIndex)
		args = append(args, parsedCursor)
		argIndex++
	}

	query += " ORDER BY l.created_at DESC LIMIT $" + fmt.Sprintf("%d", argIndex)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	defer rows.Close()

	var links []models.SectionLink
	for rows.Next() {
		var link models.SectionLink
		var metadataJSON sql.NullString
		if err := rows.Scan(&link.ID, &link.URL, &metadataJSON, &link.PostID, &link.UserID, &link.Username, &link.CreatedAt); err != nil {
			recordSpanError(span, err)
			return nil, err
		}

		if metadataJSON.Valid {
			if err := json.Unmarshal([]byte(metadataJSON.String), &link.Metadata); err != nil {
				link.Metadata = nil
			}
		}

		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	hasMore := len(links) > limit
	if hasMore {
		links = links[:limit]
	}

	var nextCursor *string
	if hasMore && len(links) > 0 {
		cursorStr := links[len(links)-1].CreatedAt.Format("2006-01-02T15:04:05.000Z07:00")
		nextCursor = &cursorStr
	}

	if links == nil {
		links = []models.SectionLink{}
	}

	return &models.SectionLinksResponse{
		Links:      links,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}
