package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// SearchService handles search operations.
type SearchService struct {
	db             *sql.DB
	postService    *PostService
	commentService *CommentService
}

// NewSearchService creates a new search service.
func NewSearchService(db *sql.DB) *SearchService {
	return &SearchService{
		db:             db,
		postService:    NewPostService(db),
		commentService: NewCommentService(db),
	}
}

// IsQueryMeaningful checks if a query produces a non-empty tsquery.
func (s *SearchService) IsQueryMeaningful(ctx context.Context, query string) (bool, error) {
	var tsquery string
	if err := s.db.QueryRowContext(ctx, "SELECT plainto_tsquery('english', $1)::text", query).Scan(&tsquery); err != nil {
		return false, err
	}
	return strings.TrimSpace(tsquery) != "", nil
}

// Search searches posts and comments, including link metadata, with optional scope filtering.
func (s *SearchService) Search(ctx context.Context, query string, scope string, sectionID *uuid.UUID, limit int, userID uuid.UUID) ([]models.SearchResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	ctx, span := otel.Tracer("clubhouse.search").Start(ctx, "SearchService.Search")
	span.SetAttributes(
		attribute.String("scope", scope),
		attribute.Int("limit", limit),
		attribute.Int("query_length", len(query)),
	)
	if sectionID != nil {
		span.SetAttributes(attribute.String("section_id", sectionID.String()))
	}
	defer span.End()

	postScopeFilter := ""
	commentScopeFilter := ""
	linkScopeFilter := ""
	args := []any{query}
	limitPlaceholder := "$2"
	if scope == "section" {
		postScopeFilter = " AND p.section_id = $2"
		commentScopeFilter = " AND p.section_id = $2"
		linkScopeFilter = " AND COALESCE(p.section_id, cp.section_id) = $2"
		args = append(args, *sectionID)
		limitPlaceholder = "$3"
	}

	queryText := fmt.Sprintf(`
		WITH q AS (SELECT plainto_tsquery('english', $1) AS query),
		post_matches AS (
			SELECT p.id,
				ts_rank_cd(p.search_vector, q.query)
				+ COALESCE(MAX(ts_rank_cd(l.search_vector, q.query)), 0) AS rank
			FROM posts p
			LEFT JOIN links l ON l.post_id = p.id
			CROSS JOIN q
			WHERE p.deleted_at IS NULL
				AND (
					p.search_vector @@ q.query
					OR l.search_vector @@ q.query
				)
				%s
			GROUP BY p.id, q.query
		),
		comment_matches AS (
			SELECT c.id,
				ts_rank_cd(c.search_vector, q.query)
				+ COALESCE(MAX(ts_rank_cd(l.search_vector, q.query)), 0) AS rank
			FROM comments c
			JOIN posts p ON c.post_id = p.id
			LEFT JOIN links l ON l.comment_id = c.id
			CROSS JOIN q
			WHERE c.deleted_at IS NULL
				AND p.deleted_at IS NULL
				AND (
					c.search_vector @@ q.query
					OR l.search_vector @@ q.query
				)
				%s
			GROUP BY c.id, q.query
		),
		link_matches AS (
			SELECT l.id,
				ts_rank_cd(l.search_vector, q.query) AS rank
			FROM links l
			LEFT JOIN posts p ON l.post_id = p.id
			LEFT JOIN comments c ON l.comment_id = c.id
			LEFT JOIN posts cp ON c.post_id = cp.id
			CROSS JOIN q
			WHERE l.search_vector @@ q.query
				AND (
					(l.post_id IS NOT NULL AND p.deleted_at IS NULL)
					OR (l.comment_id IS NOT NULL AND c.deleted_at IS NULL AND cp.deleted_at IS NULL)
				)
				%s
		)
		SELECT 'post' AS result_type, id, rank FROM post_matches
		UNION ALL
		SELECT 'comment' AS result_type, id, rank FROM comment_matches
		UNION ALL
		SELECT 'link_metadata' AS result_type, id, rank FROM link_matches
		ORDER BY rank DESC
		LIMIT %s
	`, postScopeFilter, commentScopeFilter, linkScopeFilter, limitPlaceholder)

	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, queryText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]models.SearchResult, 0)
	for rows.Next() {
		var resultType string
		var id uuid.UUID
		var rank float64

		if err := rows.Scan(&resultType, &id, &rank); err != nil {
			return nil, err
		}

		switch resultType {
		case "post":
			post, err := s.postService.GetPostByID(ctx, id, userID)
			if err != nil {
				continue
			}
			results = append(results, models.SearchResult{
				Type:  "post",
				Score: rank,
				Post:  post,
			})
		case "comment":
			comment, err := s.commentService.GetCommentByID(ctx, id, userID)
			if err != nil {
				continue
			}
			results = append(results, models.SearchResult{
				Type:    "comment",
				Score:   rank,
				Comment: comment,
			})
		case "link_metadata":
			linkResult, err := s.getLinkMetadataResult(ctx, id)
			if err != nil {
				continue
			}
			results = append(results, models.SearchResult{
				Type:         "link_metadata",
				Score:        rank,
				LinkMetadata: linkResult,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func (s *SearchService) getLinkMetadataResult(ctx context.Context, linkID uuid.UUID) (*models.LinkMetadataResult, error) {
	query := `
		SELECT id, url, metadata, post_id, comment_id
		FROM links
		WHERE id = $1
	`

	var result models.LinkMetadataResult
	var metadataBytes []byte
	if err := s.db.QueryRowContext(ctx, query, linkID).Scan(&result.ID, &result.URL, &metadataBytes, &result.PostID, &result.CommentID); err != nil {
		return nil, err
	}
	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &result.Metadata); err != nil {
			return nil, err
		}
	}

	return &result, nil
}
