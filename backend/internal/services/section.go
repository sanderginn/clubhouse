package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type SectionService struct {
	db *sql.DB
}

const recentPodcastCursorSeparator = "|"

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
			WHEN 'podcast' THEN 3
			WHEN 'movie' THEN 4
			WHEN 'series' THEN 5
			WHEN 'recipe' THEN 6
			WHEN 'book' THEN 7
			WHEN 'event' THEN 8
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

func (s *SectionService) GetRecentPodcasts(ctx context.Context, sectionID uuid.UUID, cursor *string, limit int) (*models.SectionRecentPodcastsResponse, error) {
	ctx, span := otel.Tracer("clubhouse.sections").Start(ctx, "SectionService.GetRecentPodcasts")
	span.SetAttributes(
		attribute.String("section_id", sectionID.String()),
		attribute.Int("limit", limit),
		attribute.Bool("has_cursor", cursor != nil && strings.TrimSpace(*cursor) != ""),
	)
	defer span.End()

	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	var sectionType string
	err := s.db.QueryRowContext(ctx, "SELECT type FROM sections WHERE id = $1", sectionID).Scan(&sectionType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := errors.New("section not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, err
	}
	if sectionType != "podcast" {
		invalidTypeErr := errors.New("section is not podcast")
		recordSpanError(span, invalidTypeErr)
		return nil, invalidTypeErr
	}

	query := `
		SELECT
			l.id, l.post_id, l.url, l.metadata,
			p.user_id, u.username, p.created_at, l.created_at
		FROM links l
		JOIN posts p ON l.post_id = p.id
		JOIN users u ON p.user_id = u.id
		WHERE p.section_id = $1
			AND p.deleted_at IS NULL
			AND l.metadata IS NOT NULL
			AND l.metadata ? 'podcast'
	`

	args := []interface{}{sectionID}
	argIndex := 2
	if cursor != nil && strings.TrimSpace(*cursor) != "" {
		cursorTime, cursorLinkID, err := parseRecentPodcastCursor(strings.TrimSpace(*cursor))
		if err != nil {
			invalidCursorErr := errors.New("invalid cursor")
			recordSpanError(span, invalidCursorErr)
			return nil, invalidCursorErr
		}
		query += fmt.Sprintf(" AND (l.created_at < $%d OR (l.created_at = $%d AND l.id < $%d))", argIndex, argIndex, argIndex+1)
		args = append(args, cursorTime, cursorLinkID)
		argIndex += 2
	}

	query += fmt.Sprintf(" ORDER BY l.created_at DESC, l.id DESC LIMIT $%d", argIndex)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	defer rows.Close()

	items := make([]models.RecentPodcastItem, 0, limit+1)
	for rows.Next() {
		var (
			linkID        uuid.UUID
			postID        uuid.UUID
			linkURL       string
			metadataJSON  sql.NullString
			userID        uuid.UUID
			username      string
			postCreatedAt time.Time
			linkCreatedAt time.Time
		)
		if err := rows.Scan(&linkID, &postID, &linkURL, &metadataJSON, &userID, &username, &postCreatedAt, &linkCreatedAt); err != nil {
			recordSpanError(span, err)
			return nil, err
		}
		if !metadataJSON.Valid {
			continue
		}

		metadata := make(map[string]interface{})
		if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err != nil {
			continue
		}
		podcast, err := extractPodcastFromMetadata(metadata)
		if err != nil || podcast == nil || strings.TrimSpace(podcast.Kind) == "" {
			continue
		}

		items = append(items, models.RecentPodcastItem{
			PostID:        postID,
			LinkID:        linkID,
			URL:           linkURL,
			Podcast:       *podcast,
			UserID:        userID,
			Username:      username,
			PostCreatedAt: postCreatedAt,
			LinkCreatedAt: linkCreatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	var nextCursor *string
	if hasMore && len(items) > 0 {
		cursorValue := buildRecentPodcastCursor(items[len(items)-1].LinkCreatedAt, items[len(items)-1].LinkID)
		nextCursor = &cursorValue
	}

	if items == nil {
		items = []models.RecentPodcastItem{}
	}

	return &models.SectionRecentPodcastsResponse{
		Items:      items,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

func buildRecentPodcastCursor(createdAt time.Time, linkID uuid.UUID) string {
	return createdAt.UTC().Format(time.RFC3339Nano) + recentPodcastCursorSeparator + linkID.String()
}

func parseRecentPodcastCursor(cursor string) (time.Time, uuid.UUID, error) {
	parts := strings.Split(cursor, recentPodcastCursorSeparator)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, errors.New("invalid cursor")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, errors.New("invalid cursor")
	}
	linkID, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, errors.New("invalid cursor")
	}
	return createdAt, linkID, nil
}
