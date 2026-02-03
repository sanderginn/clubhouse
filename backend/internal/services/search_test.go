package services

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestSearchServiceGlobal(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	service := NewSearchService(db)

	query := "hello world"
	limit := 5
	postID := uuid.New()
	commentID := uuid.New()
	userID := uuid.New()
	sectionID := uuid.New()
	postCreated := time.Now()
	commentCreated := time.Now()
	userCreated := time.Now()

	searchRows := sqlmock.NewRows([]string{"result_type", "id", "rank"}).
		AddRow("post", postID, 0.42).
		AddRow("comment", commentID, 0.31)

	mock.ExpectQuery(regexp.QuoteMeta("WITH q AS")).
		WithArgs(query, limit).
		WillReturnRows(searchRows)

	postRows := sqlmock.NewRows([]string{
		"id", "user_id", "section_id", "content", "created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at", "comment_count", "type",
	}).AddRow(
		postID, userID, sectionID, "post content", postCreated, nil, nil, nil,
		userID, "alice", "alice@example.com", nil, nil, false, userCreated, 0, "general",
	)

	mock.ExpectQuery(regexp.QuoteMeta("FROM posts p")).
		WithArgs(postID).
		WillReturnRows(postRows)

	mock.ExpectQuery(regexp.QuoteMeta("FROM links")).
		WithArgs(postID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url", "metadata", "created_at"}))

	mock.ExpectQuery(regexp.QuoteMeta("FROM post_images")).
		WithArgs(postID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "image_url", "position", "caption", "alt_text", "created_at"}))

	// Mock reaction counts for post
	mock.ExpectQuery(regexp.QuoteMeta("FROM reactions")).
		WithArgs(postID).
		WillReturnRows(sqlmock.NewRows([]string{"emoji", "count"}))

	commentRows := sqlmock.NewRows([]string{
		"id", "user_id", "post_id", "section_id", "parent_comment_id", "image_id", "content", "created_at", "updated_at", "deleted_at", "deleted_by_user_id",
		"id", "username", "email", "profile_picture_url", "bio", "is_admin", "created_at",
	}).AddRow(
		commentID, userID, postID, sectionID, nil, nil, "comment content", commentCreated, nil, nil, nil,
		userID, "alice", "alice@example.com", nil, nil, false, userCreated,
	)

	mock.ExpectQuery(regexp.QuoteMeta("FROM comments c")).
		WithArgs(commentID).
		WillReturnRows(commentRows)

	mock.ExpectQuery(regexp.QuoteMeta("FROM links")).
		WithArgs(commentID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url", "metadata", "created_at"}))

	// Mock reaction counts for comment
	mock.ExpectQuery(regexp.QuoteMeta("FROM reactions")).
		WithArgs(commentID).
		WillReturnRows(sqlmock.NewRows([]string{"emoji", "count"}))

	results, err := service.Search(context.Background(), query, "global", nil, limit, uuid.Nil)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}

func TestSearchServiceSectionScope(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	service := NewSearchService(db)

	query := "hello"
	limit := 10
	sectionID := uuid.New()

	searchRows := sqlmock.NewRows([]string{"result_type", "id", "rank"})

	mock.ExpectQuery(regexp.QuoteMeta("WITH q AS")).
		WithArgs(query, sectionID, limit).
		WillReturnRows(searchRows)

	results, err := service.Search(context.Background(), query, "section", &sectionID, limit, uuid.Nil)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}
