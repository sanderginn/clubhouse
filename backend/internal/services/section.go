package services

import (
	"context"
	"database/sql"
	"errors"

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
