package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const (
	defaultBookQuoteLimit       = 20
	maxBookQuoteLimit           = 100
	bookQuoteLegacyCursorLayout = "2006-01-02T15:04:05.000Z07:00"
	bookQuoteCursorSeparator    = "|"
)

// BookQuoteService handles CRUD operations for book quotes.
type BookQuoteService struct {
	db *sql.DB
}

// NewBookQuoteService creates a new book quote service.
func NewBookQuoteService(db *sql.DB) *BookQuoteService {
	return &BookQuoteService{db: db}
}

// CreateQuote creates a new quote for a book post.
func (s *BookQuoteService) CreateQuote(
	ctx context.Context,
	userID uuid.UUID,
	postID uuid.UUID,
	req models.CreateBookQuoteRequest,
) (*models.BookQuoteWithUser, error) {
	ctx, span := otel.Tracer("clubhouse.book_quotes").Start(ctx, "BookQuoteService.CreateQuote")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("post_id", postID.String()),
		attribute.Bool("has_page_number", req.PageNumber != nil),
		attribute.Bool("has_chapter", req.Chapter != nil && strings.TrimSpace(*req.Chapter) != ""),
		attribute.Bool("has_note", req.Note != nil && strings.TrimSpace(*req.Note) != ""),
	)
	defer span.End()

	quoteText := strings.TrimSpace(req.QuoteText)
	if quoteText == "" {
		err := errors.New("quote text is required")
		recordSpanError(span, err)
		return nil, err
	}

	if err := s.verifyBookPost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	chapter := sanitizeOptionalBookQuoteText(req.Chapter)
	note := sanitizeOptionalBookQuoteText(req.Note)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	quoteID := uuid.New()
	query := `
		INSERT INTO book_quotes (id, post_id, user_id, quote_text, page_number, chapter, note, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now(), now())
		RETURNING id, post_id, user_id, quote_text, page_number, chapter, note, created_at, updated_at, deleted_at
	`

	row := tx.QueryRowContext(
		ctx,
		query,
		quoteID,
		postID,
		userID,
		quoteText,
		nullableInt(req.PageNumber),
		nullableString(chapter),
		nullableString(note),
	)
	createdQuote, err := scanBookQuote(row)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to create book quote: %w", err)
	}

	username, displayName, err := getBookQuoteUserSummary(ctx, tx, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	quote := withBookQuoteUser(createdQuote, username, displayName)

	metadata := map[string]interface{}{
		"quote_id":        quote.ID.String(),
		"post_id":         quote.PostID.String(),
		"quote_excerpt":   truncateAuditExcerpt(quoteText),
		"has_page_number": req.PageNumber != nil,
		"has_chapter":     chapter != nil,
		"has_note":        note != nil,
	}
	if req.PageNumber != nil {
		metadata["page_number"] = *req.PageNumber
	}
	if chapter != nil {
		metadata["chapter"] = *chapter
	}
	if note != nil {
		metadata["note"] = *note
	}

	auditService := NewAuditService(tx)
	if err := auditService.LogAuditWithMetadata(ctx, "create_book_quote", userID, userID, metadata); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &quote, nil
}

// UpdateQuote updates an existing quote.
func (s *BookQuoteService) UpdateQuote(
	ctx context.Context,
	userID uuid.UUID,
	quoteID uuid.UUID,
	req models.UpdateBookQuoteRequest,
) (*models.BookQuoteWithUser, error) {
	ctx, span := otel.Tracer("clubhouse.book_quotes").Start(ctx, "BookQuoteService.UpdateQuote")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("quote_id", quoteID.String()),
		attribute.Bool("has_quote_text", req.QuoteText != nil),
		attribute.Bool("has_page_number", req.PageNumber != nil),
		attribute.Bool("has_chapter", req.Chapter != nil),
		attribute.Bool("has_note", req.Note != nil),
	)
	defer span.End()

	existing, err := s.getQuoteByID(ctx, quoteID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	isAdmin, err := s.isAdmin(ctx, userID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	if existing.UserID != userID && !isAdmin {
		unauthorizedErr := errors.New("unauthorized to edit this quote")
		recordSpanError(span, unauthorizedErr)
		return nil, unauthorizedErr
	}

	quoteText := existing.QuoteText
	if req.QuoteText != nil {
		trimmed := strings.TrimSpace(*req.QuoteText)
		if trimmed == "" {
			err := errors.New("quote text is required")
			recordSpanError(span, err)
			return nil, err
		}
		quoteText = trimmed
	}

	pageNumber := existing.PageNumber
	if req.PageNumber != nil {
		updated := *req.PageNumber
		pageNumber = &updated
	}

	chapter := existing.Chapter
	if req.Chapter != nil {
		chapter = sanitizeOptionalBookQuoteText(req.Chapter)
	}

	note := existing.Note
	if req.Note != nil {
		note = sanitizeOptionalBookQuoteText(req.Note)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	query := `
		UPDATE book_quotes
		SET quote_text = $2, page_number = $3, chapter = $4, note = $5, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, post_id, user_id, quote_text, page_number, chapter, note, created_at, updated_at, deleted_at
	`
	row := tx.QueryRowContext(
		ctx,
		query,
		quoteID,
		quoteText,
		nullableInt(pageNumber),
		nullableString(chapter),
		nullableString(note),
	)

	updatedBookQuote, err := scanBookQuote(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFoundErr := errors.New("book quote not found")
			recordSpanError(span, notFoundErr)
			return nil, notFoundErr
		}
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to update book quote: %w", err)
	}

	username, displayName, err := getBookQuoteUserSummary(ctx, tx, updatedBookQuote.UserID)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	updatedQuote := withBookQuoteUser(updatedBookQuote, username, displayName)

	metadata := map[string]interface{}{
		"quote_id":            updatedQuote.ID.String(),
		"post_id":             updatedQuote.PostID.String(),
		"quote_excerpt":       truncateAuditExcerpt(updatedQuote.QuoteText),
		"previous_quote_text": existing.QuoteText,
	}
	if req.PageNumber != nil {
		metadata["page_number"] = nullableInt(pageNumber)
	}
	if req.Chapter != nil {
		metadata["chapter"] = nullableString(chapter)
	}
	if req.Note != nil {
		metadata["note"] = nullableString(note)
	}
	if existing.UserID != userID && isAdmin {
		metadata["updated_by_admin"] = true
	}

	auditService := NewAuditService(tx)
	if err := auditService.LogAuditWithMetadata(ctx, "update_book_quote", userID, existing.UserID, metadata); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &updatedQuote, nil
}

// DeleteQuote soft deletes an existing quote.
func (s *BookQuoteService) DeleteQuote(ctx context.Context, userID uuid.UUID, quoteID uuid.UUID) error {
	ctx, span := otel.Tracer("clubhouse.book_quotes").Start(ctx, "BookQuoteService.DeleteQuote")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.String("quote_id", quoteID.String()),
	)
	defer span.End()

	existing, err := s.getQuoteByID(ctx, quoteID)
	if err != nil {
		recordSpanError(span, err)
		return err
	}

	isAdmin, err := s.isAdmin(ctx, userID)
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	if existing.UserID != userID && !isAdmin {
		unauthorizedErr := errors.New("unauthorized to delete this quote")
		recordSpanError(span, unauthorizedErr)
		return unauthorizedErr
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	result, err := tx.ExecContext(
		ctx,
		`UPDATE book_quotes SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`,
		quoteID,
	)
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to delete book quote: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to verify book quote deletion: %w", err)
	}
	if rowsAffected == 0 {
		notFoundErr := errors.New("book quote not found")
		recordSpanError(span, notFoundErr)
		return notFoundErr
	}

	isSelfDelete := existing.UserID == userID
	metadata := map[string]interface{}{
		"quote_id":           existing.ID.String(),
		"post_id":            existing.PostID.String(),
		"quote_excerpt":      truncateAuditExcerpt(existing.QuoteText),
		"deleted_by_user_id": userID.String(),
		"is_self_delete":     isSelfDelete,
	}
	if !isSelfDelete && isAdmin {
		metadata["deleted_by_admin"] = true
	}

	auditService := NewAuditService(tx)
	if err := auditService.LogAuditWithMetadata(ctx, "delete_book_quote", userID, existing.UserID, metadata); err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetQuotesForPost returns paginated quotes for a post.
func (s *BookQuoteService) GetQuotesForPost(
	ctx context.Context,
	postID uuid.UUID,
	cursor *string,
	limit int,
) (*models.BookQuotesListResponse, error) {
	ctx, span := otel.Tracer("clubhouse.book_quotes").Start(ctx, "BookQuoteService.GetQuotesForPost")
	span.SetAttributes(
		attribute.String("post_id", postID.String()),
		attribute.Int("limit", limit),
		attribute.Bool("has_cursor", cursor != nil && strings.TrimSpace(*cursor) != ""),
	)
	defer span.End()

	if err := s.verifyBookPost(ctx, postID); err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	response, err := s.listQuotes(ctx, "bq.post_id = $1", []interface{}{postID}, cursor, limit)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return response, nil
}

// GetQuotesByUser returns paginated quotes created by a specific user.
func (s *BookQuoteService) GetQuotesByUser(
	ctx context.Context,
	userID uuid.UUID,
	cursor *string,
	limit int,
) (*models.BookQuotesListResponse, error) {
	ctx, span := otel.Tracer("clubhouse.book_quotes").Start(ctx, "BookQuoteService.GetQuotesByUser")
	span.SetAttributes(
		attribute.String("user_id", userID.String()),
		attribute.Int("limit", limit),
		attribute.Bool("has_cursor", cursor != nil && strings.TrimSpace(*cursor) != ""),
	)
	defer span.End()

	response, err := s.listQuotes(ctx, "bq.user_id = $1", []interface{}{userID}, cursor, limit)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return response, nil
}

func (s *BookQuoteService) listQuotes(
	ctx context.Context,
	filterClause string,
	filterArgs []interface{},
	cursor *string,
	limit int,
) (*models.BookQuotesListResponse, error) {
	if limit <= 0 || limit > maxBookQuoteLimit {
		limit = defaultBookQuoteLimit
	}

	query := `
		SELECT
			bq.id, bq.post_id, bq.user_id, bq.quote_text, bq.page_number, bq.chapter, bq.note,
			bq.created_at, bq.updated_at, bq.deleted_at,
			u.username
		FROM book_quotes bq
		JOIN users u ON bq.user_id = u.id
		WHERE ` + filterClause + ` AND bq.deleted_at IS NULL
	`

	args := append([]interface{}{}, filterArgs...)
	argIndex := len(args) + 1

	if cursor != nil && strings.TrimSpace(*cursor) != "" {
		cursorCreatedAt, cursorQuoteID, hasQuoteID, err := parseBookQuoteCursor(strings.TrimSpace(*cursor))
		if err != nil {
			return nil, err
		}

		if hasQuoteID {
			query += fmt.Sprintf(
				" AND (bq.created_at < $%d OR (bq.created_at = $%d AND bq.id < $%d))",
				argIndex,
				argIndex,
				argIndex+1,
			)
			args = append(args, cursorCreatedAt, cursorQuoteID)
			argIndex += 2
		} else {
			query += fmt.Sprintf(" AND bq.created_at < $%d", argIndex)
			args = append(args, cursorCreatedAt)
			argIndex++
		}
	}

	query += fmt.Sprintf(" ORDER BY bq.created_at DESC, bq.id DESC LIMIT $%d", argIndex)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query book quotes: %w", err)
	}
	defer rows.Close()

	quotes := make([]models.BookQuoteWithUser, 0, limit+1)
	for rows.Next() {
		quote, err := scanBookQuoteWithUser(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan book quote: %w", err)
		}
		quotes = append(quotes, *quote)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate book quotes: %w", err)
	}

	hasMore := len(quotes) > limit
	if hasMore {
		quotes = quotes[:limit]
	}

	var nextCursor *string
	if hasMore && len(quotes) > 0 {
		cursorValue := buildBookQuoteCursor(quotes[len(quotes)-1].CreatedAt, quotes[len(quotes)-1].ID)
		nextCursor = &cursorValue
	}

	return &models.BookQuotesListResponse{
		Quotes:     quotes,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func (s *BookQuoteService) getQuoteByID(ctx context.Context, quoteID uuid.UUID) (*models.BookQuote, error) {
	query := `
		SELECT id, post_id, user_id, quote_text, page_number, chapter, note, created_at, updated_at, deleted_at
		FROM book_quotes
		WHERE id = $1 AND deleted_at IS NULL
	`
	row := s.db.QueryRowContext(ctx, query, quoteID)
	quote, err := scanBookQuote(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("book quote not found")
		}
		return nil, fmt.Errorf("failed to fetch book quote: %w", err)
	}
	return quote, nil
}

func (s *BookQuoteService) verifyBookPost(ctx context.Context, postID uuid.UUID) error {
	var sectionType string
	query := `
		SELECT s.type
		FROM posts p
		JOIN sections s ON p.section_id = s.id
		WHERE p.id = $1 AND p.deleted_at IS NULL
	`
	if err := s.db.QueryRowContext(ctx, query, postID).Scan(&sectionType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("post not found")
		}
		return fmt.Errorf("failed to verify book post: %w", err)
	}
	if sectionType != "book" {
		return errors.New("post is not a book")
	}
	return nil
}

func (s *BookQuoteService) isAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	var isAdmin bool
	if err := s.db.QueryRowContext(
		ctx,
		"SELECT is_admin FROM users WHERE id = $1 AND deleted_at IS NULL",
		userID,
	).Scan(&isAdmin); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, errors.New("user not found")
		}
		return false, fmt.Errorf("failed to verify user role: %w", err)
	}
	return isAdmin, nil
}

func getBookQuoteUserSummary(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}, userID uuid.UUID) (string, string, error) {
	var username string
	if err := q.QueryRowContext(
		ctx,
		"SELECT username FROM users WHERE id = $1 AND deleted_at IS NULL",
		userID,
	).Scan(&username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", errors.New("user not found")
		}
		return "", "", fmt.Errorf("failed to fetch quote user: %w", err)
	}

	return username, username, nil
}

func scanBookQuote(scanner interface {
	Scan(dest ...interface{}) error
}) (*models.BookQuote, error) {
	var quote models.BookQuote
	var pageNumber sql.NullInt64
	var chapter sql.NullString
	var note sql.NullString
	var deletedAt sql.NullTime

	if err := scanner.Scan(
		&quote.ID,
		&quote.PostID,
		&quote.UserID,
		&quote.QuoteText,
		&pageNumber,
		&chapter,
		&note,
		&quote.CreatedAt,
		&quote.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}

	if pageNumber.Valid {
		value := int(pageNumber.Int64)
		quote.PageNumber = &value
	}
	if chapter.Valid {
		value := chapter.String
		quote.Chapter = &value
	}
	if note.Valid {
		value := note.String
		quote.Note = &value
	}
	if deletedAt.Valid {
		quote.DeletedAt = &deletedAt.Time
	}

	return &quote, nil
}

func scanBookQuoteWithUser(scanner interface {
	Scan(dest ...interface{}) error
}) (*models.BookQuoteWithUser, error) {
	var quote models.BookQuoteWithUser
	var pageNumber sql.NullInt64
	var chapter sql.NullString
	var note sql.NullString
	var deletedAt sql.NullTime
	var username string

	if err := scanner.Scan(
		&quote.ID,
		&quote.PostID,
		&quote.UserID,
		&quote.QuoteText,
		&pageNumber,
		&chapter,
		&note,
		&quote.CreatedAt,
		&quote.UpdatedAt,
		&deletedAt,
		&username,
	); err != nil {
		return nil, err
	}

	if pageNumber.Valid {
		value := int(pageNumber.Int64)
		quote.PageNumber = &value
	}
	if chapter.Valid {
		value := chapter.String
		quote.Chapter = &value
	}
	if note.Valid {
		value := note.String
		quote.Note = &value
	}
	if deletedAt.Valid {
		quote.DeletedAt = &deletedAt.Time
	}
	quote.Username = username
	quote.DisplayName = username

	return &quote, nil
}

func withBookQuoteUser(quote *models.BookQuote, username, displayName string) models.BookQuoteWithUser {
	return models.BookQuoteWithUser{
		BookQuote:   *quote,
		Username:    username,
		DisplayName: displayName,
	}
}

func buildBookQuoteCursor(createdAt time.Time, quoteID uuid.UUID) string {
	return createdAt.UTC().Format(time.RFC3339Nano) + bookQuoteCursorSeparator + quoteID.String()
}

func parseBookQuoteCursor(cursor string) (time.Time, uuid.UUID, bool, error) {
	parts := strings.Split(cursor, bookQuoteCursorSeparator)
	if len(parts) == 2 {
		createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return time.Time{}, uuid.Nil, false, errors.New("invalid cursor")
		}

		quoteID, err := uuid.Parse(parts[1])
		if err != nil {
			return time.Time{}, uuid.Nil, false, errors.New("invalid cursor")
		}

		return createdAt.UTC(), quoteID, true, nil
	}

	createdAt, err := time.Parse(bookQuoteLegacyCursorLayout, cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, false, errors.New("invalid cursor")
	}

	return createdAt.UTC(), uuid.Nil, false, nil
}

func nullableInt(value *int) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func nullableString(value *string) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func sanitizeOptionalBookQuoteText(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}
