package services

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
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

// Search searches posts and comments, including link metadata, with optional scope filtering.
func (s *SearchService) Search(ctx context.Context, query string, scope string, sectionID *uuid.UUID, limit int) ([]models.SearchResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	linkText := "COALESCE(l.metadata->>'title','') || ' ' || COALESCE(l.metadata->>'description','') || ' ' || COALESCE(l.metadata->>'author','') || ' ' || COALESCE(l.metadata->>'artist','') || ' ' || COALESCE(l.metadata->>'provider','') || ' ' || COALESCE(l.url,'')"

	postScopeFilter := ""
	commentScopeFilter := ""
	args := []any{query}
	limitPlaceholder := "$2"
	if scope == "section" {
		postScopeFilter = " AND p.section_id = $2"
		commentScopeFilter = " AND p.section_id = $2"
		args = append(args, *sectionID)
		limitPlaceholder = "$3"
	}

	queryText := fmt.Sprintf(`
		WITH q AS (SELECT plainto_tsquery('english', $1) AS query),
		post_matches AS (
			SELECT p.id,
				ts_rank_cd(to_tsvector('english', p.content), q.query)
				+ COALESCE(MAX(ts_rank_cd(to_tsvector('english', %s), q.query)), 0) AS rank
			FROM posts p
			LEFT JOIN links l ON l.post_id = p.id
			CROSS JOIN q
			WHERE p.deleted_at IS NULL
				AND (
					to_tsvector('english', p.content) @@ q.query
					OR to_tsvector('english', %s) @@ q.query
				)
				%s
			GROUP BY p.id
		),
		comment_matches AS (
			SELECT c.id,
				ts_rank_cd(to_tsvector('english', c.content), q.query)
				+ COALESCE(MAX(ts_rank_cd(to_tsvector('english', %s), q.query)), 0) AS rank
			FROM comments c
			JOIN posts p ON c.post_id = p.id
			LEFT JOIN links l ON l.comment_id = c.id
			CROSS JOIN q
			WHERE c.deleted_at IS NULL
				AND p.deleted_at IS NULL
				AND (
					to_tsvector('english', c.content) @@ q.query
					OR to_tsvector('english', %s) @@ q.query
				)
				%s
			GROUP BY c.id
		)
		SELECT 'post' AS result_type, id, rank FROM post_matches
		UNION ALL
		SELECT 'comment' AS result_type, id, rank FROM comment_matches
		ORDER BY rank DESC
		LIMIT %s
	`, linkText, linkText, postScopeFilter, linkText, linkText, commentScopeFilter, limitPlaceholder)

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
			post, err := s.postService.GetPostByID(ctx, id)
			if err != nil {
				continue
			}
			results = append(results, models.SearchResult{
				Type:  "post",
				Score: rank,
				Post:  post,
			})
		case "comment":
			comment, err := s.commentService.GetCommentByID(ctx, id)
			if err != nil {
				continue
			}
			results = append(results, models.SearchResult{
				Type:    "comment",
				Score:   rank,
				Comment: comment,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}
