package services

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
)

type SectionService struct {
	db *sql.DB
}

func NewSectionService(db *sql.DB) *SectionService {
	return &SectionService{db: db}
}

func (s *SectionService) ListSections(ctx context.Context) ([]models.Section, error) {
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
		return nil, err
	}
	defer rows.Close()

	var sections []models.Section
	for rows.Next() {
		var section models.Section
		if err := rows.Scan(&section.ID, &section.Name, &section.Type); err != nil {
			return nil, err
		}
		sections = append(sections, section)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if sections == nil {
		sections = []models.Section{}
	}

	return sections, nil
}

func (s *SectionService) GetSectionByID(ctx context.Context, id uuid.UUID) (*models.Section, error) {
	query := `SELECT id, name, type FROM sections WHERE id = $1`

	var section models.Section
	err := s.db.QueryRowContext(ctx, query, id).Scan(&section.ID, &section.Name, &section.Type)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("section not found")
		}
		return nil, err
	}

	return &section, nil
}
