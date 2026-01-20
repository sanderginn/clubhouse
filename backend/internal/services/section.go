package services

import (
	"context"
	"database/sql"

	"github.com/sanderginn/clubhouse/internal/models"
)

type SectionService struct {
	db *sql.DB
}

func NewSectionService(db *sql.DB) *SectionService {
	return &SectionService{db: db}
}

func (s *SectionService) ListSections(ctx context.Context) ([]models.Section, error) {
	query := `SELECT id, name, type FROM sections ORDER BY name ASC`

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
