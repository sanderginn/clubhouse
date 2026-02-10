package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestCreateQuoteWithAllFields(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquoteall", "bookquoteall@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book quote post"))

	service := NewBookQuoteService(db)
	quote, err := service.CreateQuote(context.Background(), userID, postID, models.CreateBookQuoteRequest{
		QuoteText:  "  A room without books is like a body without a soul.  ",
		PageNumber: intPtr(42),
		Chapter:    stringPtr("  Chapter 2  "),
		Note:       stringPtr("  Favorite quote  "),
	})
	if err != nil {
		t.Fatalf("CreateQuote failed: %v", err)
	}

	if quote.QuoteText != "A room without books is like a body without a soul." {
		t.Fatalf("expected trimmed quote text, got %q", quote.QuoteText)
	}
	if quote.PageNumber == nil || *quote.PageNumber != 42 {
		t.Fatalf("expected page number 42, got %v", quote.PageNumber)
	}
	if quote.Chapter == nil || *quote.Chapter != "Chapter 2" {
		t.Fatalf("expected chapter %q, got %v", "Chapter 2", quote.Chapter)
	}
	if quote.Note == nil || *quote.Note != "Favorite quote" {
		t.Fatalf("expected note %q, got %v", "Favorite quote", quote.Note)
	}
	if quote.Username != "bookquoteall" {
		t.Fatalf("expected username %q, got %q", "bookquoteall", quote.Username)
	}
	if quote.DisplayName != "bookquoteall" {
		t.Fatalf("expected display name %q, got %q", "bookquoteall", quote.DisplayName)
	}
}

func TestCreateQuoteWithMinimalFields(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquotemin", "bookquotemin@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book quote post"))

	service := NewBookQuoteService(db)
	quote, err := service.CreateQuote(context.Background(), userID, postID, models.CreateBookQuoteRequest{
		QuoteText: "Minimal quote",
	})
	if err != nil {
		t.Fatalf("CreateQuote failed: %v", err)
	}

	if quote.QuoteText != "Minimal quote" {
		t.Fatalf("expected quote text %q, got %q", "Minimal quote", quote.QuoteText)
	}
	if quote.PageNumber != nil {
		t.Fatalf("expected nil page number, got %v", quote.PageNumber)
	}
	if quote.Chapter != nil {
		t.Fatalf("expected nil chapter, got %v", quote.Chapter)
	}
	if quote.Note != nil {
		t.Fatalf("expected nil note, got %v", quote.Note)
	}
}

func TestUpdateQuote(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquoteupdate", "bookquoteupdate@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book quote post"))

	service := NewBookQuoteService(db)
	created, err := service.CreateQuote(context.Background(), userID, postID, models.CreateBookQuoteRequest{
		QuoteText:  "Original quote",
		PageNumber: intPtr(10),
		Chapter:    stringPtr("Chapter 1"),
		Note:       stringPtr("Original note"),
	})
	if err != nil {
		t.Fatalf("CreateQuote failed: %v", err)
	}

	updated, err := service.UpdateQuote(context.Background(), userID, created.ID, models.UpdateBookQuoteRequest{
		QuoteText:  stringPtr("Updated quote"),
		PageNumber: intPtr(25),
		Note:       stringPtr("Updated note"),
	})
	if err != nil {
		t.Fatalf("UpdateQuote failed: %v", err)
	}

	if updated.QuoteText != "Updated quote" {
		t.Fatalf("expected quote text %q, got %q", "Updated quote", updated.QuoteText)
	}
	if updated.PageNumber == nil || *updated.PageNumber != 25 {
		t.Fatalf("expected page number 25, got %v", updated.PageNumber)
	}
	if updated.Chapter == nil || *updated.Chapter != "Chapter 1" {
		t.Fatalf("expected chapter to remain %q, got %v", "Chapter 1", updated.Chapter)
	}
	if updated.Note == nil || *updated.Note != "Updated note" {
		t.Fatalf("expected note %q, got %v", "Updated note", updated.Note)
	}
}

func TestDeleteOwnQuote(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquotedelete", "bookquotedelete@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book quote post"))

	service := NewBookQuoteService(db)
	created, err := service.CreateQuote(context.Background(), userID, postID, models.CreateBookQuoteRequest{
		QuoteText: "Delete me",
	})
	if err != nil {
		t.Fatalf("CreateQuote failed: %v", err)
	}

	if err := service.DeleteQuote(context.Background(), userID, created.ID); err != nil {
		t.Fatalf("DeleteQuote failed: %v", err)
	}

	var deletedAt time.Time
	if err := db.QueryRowContext(context.Background(), `
		SELECT deleted_at
		FROM book_quotes
		WHERE id = $1
	`, created.ID).Scan(&deletedAt); err != nil {
		t.Fatalf("failed to query deleted quote: %v", err)
	}
	if deletedAt.IsZero() {
		t.Fatalf("expected deleted_at to be set")
	}
}

func TestAdminDeleteAnyQuote(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	ownerID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquoteowner", "bookquoteowner@test.com", false, true))
	adminID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquoteadmin", "bookquoteadmin@test.com", true, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, ownerID.String(), sectionID, "Book quote post"))

	service := NewBookQuoteService(db)
	created, err := service.CreateQuote(context.Background(), ownerID, postID, models.CreateBookQuoteRequest{
		QuoteText: "Admin can delete this",
	})
	if err != nil {
		t.Fatalf("CreateQuote failed: %v", err)
	}

	if err := service.DeleteQuote(context.Background(), adminID, created.ID); err != nil {
		t.Fatalf("DeleteQuote failed: %v", err)
	}

	var deletedAt time.Time
	if err := db.QueryRowContext(context.Background(), `
		SELECT deleted_at
		FROM book_quotes
		WHERE id = $1
	`, created.ID).Scan(&deletedAt); err != nil {
		t.Fatalf("failed to query deleted quote: %v", err)
	}
	if deletedAt.IsZero() {
		t.Fatalf("expected deleted_at to be set")
	}
}

func TestNonOwnerCannotEditOrDeleteQuote(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	ownerID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquoteowner2", "bookquoteowner2@test.com", false, true))
	otherUserID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquoteother", "bookquoteother@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, ownerID.String(), sectionID, "Book quote post"))

	service := NewBookQuoteService(db)
	created, err := service.CreateQuote(context.Background(), ownerID, postID, models.CreateBookQuoteRequest{
		QuoteText: "Protected quote",
	})
	if err != nil {
		t.Fatalf("CreateQuote failed: %v", err)
	}

	_, err = service.UpdateQuote(context.Background(), otherUserID, created.ID, models.UpdateBookQuoteRequest{
		QuoteText: stringPtr("Unauthorized edit"),
	})
	if err == nil {
		t.Fatalf("expected update error for non-owner")
	}
	if err.Error() != "unauthorized to edit this quote" {
		t.Fatalf("expected unauthorized edit error, got %q", err.Error())
	}

	err = service.DeleteQuote(context.Background(), otherUserID, created.ID)
	if err == nil {
		t.Fatalf("expected delete error for non-owner")
	}
	if err.Error() != "unauthorized to delete this quote" {
		t.Fatalf("expected unauthorized delete error, got %q", err.Error())
	}
}

func TestBookQuotePagination(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquotepage", "bookquotepage@test.com", false, true))
	otherUserID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquotepage2", "bookquotepage2@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book quote post"))
	otherPostID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Another book quote post"))

	service := NewBookQuoteService(db)
	oldest, err := service.CreateQuote(context.Background(), userID, postID, models.CreateBookQuoteRequest{QuoteText: "Oldest"})
	if err != nil {
		t.Fatalf("CreateQuote oldest failed: %v", err)
	}
	middle, err := service.CreateQuote(context.Background(), userID, postID, models.CreateBookQuoteRequest{QuoteText: "Middle"})
	if err != nil {
		t.Fatalf("CreateQuote middle failed: %v", err)
	}
	newest, err := service.CreateQuote(context.Background(), userID, postID, models.CreateBookQuoteRequest{QuoteText: "Newest"})
	if err != nil {
		t.Fatalf("CreateQuote newest failed: %v", err)
	}
	if _, err := service.CreateQuote(context.Background(), otherUserID, otherPostID, models.CreateBookQuoteRequest{QuoteText: "Other user quote"}); err != nil {
		t.Fatalf("CreateQuote other user failed: %v", err)
	}

	now := time.Now().UTC()
	if _, err := db.ExecContext(context.Background(), `UPDATE book_quotes SET created_at = $1 WHERE id = $2`, now.Add(-3*time.Hour), oldest.ID); err != nil {
		t.Fatalf("failed to set oldest created_at: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `UPDATE book_quotes SET created_at = $1 WHERE id = $2`, now.Add(-2*time.Hour), middle.ID); err != nil {
		t.Fatalf("failed to set middle created_at: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `UPDATE book_quotes SET created_at = $1 WHERE id = $2`, now.Add(-1*time.Hour), newest.ID); err != nil {
		t.Fatalf("failed to set newest created_at: %v", err)
	}

	firstPage, err := service.GetQuotesForPost(context.Background(), postID, nil, 2)
	if err != nil {
		t.Fatalf("GetQuotesForPost first page failed: %v", err)
	}
	if len(firstPage.Quotes) != 2 {
		t.Fatalf("expected 2 quotes on first page, got %d", len(firstPage.Quotes))
	}
	if !firstPage.HasMore {
		t.Fatalf("expected HasMore on first page")
	}
	if firstPage.NextCursor == nil || *firstPage.NextCursor == "" {
		t.Fatalf("expected next cursor on first page")
	}
	if firstPage.Quotes[0].ID != newest.ID {
		t.Fatalf("expected newest quote first, got %s", firstPage.Quotes[0].ID)
	}
	if firstPage.Quotes[1].ID != middle.ID {
		t.Fatalf("expected middle quote second, got %s", firstPage.Quotes[1].ID)
	}

	secondPage, err := service.GetQuotesForPost(context.Background(), postID, firstPage.NextCursor, 2)
	if err != nil {
		t.Fatalf("GetQuotesForPost second page failed: %v", err)
	}
	if len(secondPage.Quotes) != 1 {
		t.Fatalf("expected 1 quote on second page, got %d", len(secondPage.Quotes))
	}
	if secondPage.HasMore {
		t.Fatalf("expected no more pages after second page")
	}
	if secondPage.NextCursor != nil {
		t.Fatalf("expected nil next cursor on final page")
	}
	if secondPage.Quotes[0].ID != oldest.ID {
		t.Fatalf("expected oldest quote on second page, got %s", secondPage.Quotes[0].ID)
	}

	byUser, err := service.GetQuotesByUser(context.Background(), userID, nil, 10)
	if err != nil {
		t.Fatalf("GetQuotesByUser failed: %v", err)
	}
	if len(byUser.Quotes) != 3 {
		t.Fatalf("expected 3 quotes for user, got %d", len(byUser.Quotes))
	}
}

func TestBookQuoteAuditLogEntries(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	ownerID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquoteaudit", "bookquoteaudit@test.com", false, true))
	adminID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquoteauditadmin", "bookquoteauditadmin@test.com", true, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, ownerID.String(), sectionID, "Book quote post"))

	service := NewBookQuoteService(db)
	created, err := service.CreateQuote(context.Background(), ownerID, postID, models.CreateBookQuoteRequest{
		QuoteText: "Audit quote",
	})
	if err != nil {
		t.Fatalf("CreateQuote failed: %v", err)
	}

	_, err = service.UpdateQuote(context.Background(), ownerID, created.ID, models.UpdateBookQuoteRequest{
		QuoteText: stringPtr("Audit quote updated"),
	})
	if err != nil {
		t.Fatalf("UpdateQuote failed: %v", err)
	}

	if err := service.DeleteQuote(context.Background(), adminID, created.ID); err != nil {
		t.Fatalf("DeleteQuote failed: %v", err)
	}

	var createMetadataBytes []byte
	var createAdminID uuid.UUID
	var createTargetID uuid.NullUUID
	err = db.QueryRowContext(context.Background(), `
		SELECT admin_user_id, target_user_id, metadata
		FROM audit_logs
		WHERE action = 'create_book_quote'
	`).Scan(&createAdminID, &createTargetID, &createMetadataBytes)
	if err != nil {
		t.Fatalf("failed to query create audit log: %v", err)
	}
	if createAdminID != ownerID {
		t.Fatalf("expected create admin_user_id %s, got %s", ownerID, createAdminID)
	}
	if !createTargetID.Valid || createTargetID.UUID != ownerID {
		t.Fatalf("expected create target_user_id %s, got %v", ownerID, createTargetID)
	}

	createMetadata := parseAuditMetadata(t, createMetadataBytes)
	if createMetadata["quote_id"] != created.ID.String() {
		t.Fatalf("expected create quote_id %s, got %v", created.ID, createMetadata["quote_id"])
	}
	if createMetadata["post_id"] != postID.String() {
		t.Fatalf("expected create post_id %s, got %v", postID, createMetadata["post_id"])
	}

	var updateMetadataBytes []byte
	var updateAdminID uuid.UUID
	err = db.QueryRowContext(context.Background(), `
		SELECT admin_user_id, metadata
		FROM audit_logs
		WHERE action = 'update_book_quote'
	`).Scan(&updateAdminID, &updateMetadataBytes)
	if err != nil {
		t.Fatalf("failed to query update audit log: %v", err)
	}
	if updateAdminID != ownerID {
		t.Fatalf("expected update admin_user_id %s, got %s", ownerID, updateAdminID)
	}

	updateMetadata := parseAuditMetadata(t, updateMetadataBytes)
	if updateMetadata["quote_id"] != created.ID.String() {
		t.Fatalf("expected update quote_id %s, got %v", created.ID, updateMetadata["quote_id"])
	}
	if updateMetadata["previous_quote_text"] != "Audit quote" {
		t.Fatalf("expected previous_quote_text %q, got %v", "Audit quote", updateMetadata["previous_quote_text"])
	}

	var deleteMetadataBytes []byte
	var deleteAdminID uuid.UUID
	var deleteTargetID uuid.NullUUID
	err = db.QueryRowContext(context.Background(), `
		SELECT admin_user_id, target_user_id, metadata
		FROM audit_logs
		WHERE action = 'delete_book_quote'
	`).Scan(&deleteAdminID, &deleteTargetID, &deleteMetadataBytes)
	if err != nil {
		t.Fatalf("failed to query delete audit log: %v", err)
	}
	if deleteAdminID != adminID {
		t.Fatalf("expected delete admin_user_id %s, got %s", adminID, deleteAdminID)
	}
	if !deleteTargetID.Valid || deleteTargetID.UUID != ownerID {
		t.Fatalf("expected delete target_user_id %s, got %v", ownerID, deleteTargetID)
	}

	deleteMetadata := parseAuditMetadata(t, deleteMetadataBytes)
	if deleteMetadata["quote_id"] != created.ID.String() {
		t.Fatalf("expected delete quote_id %s, got %v", created.ID, deleteMetadata["quote_id"])
	}
	deletedByAdmin, ok := deleteMetadata["deleted_by_admin"].(bool)
	if !ok || !deletedByAdmin {
		t.Fatalf("expected deleted_by_admin true, got %v", deleteMetadata["deleted_by_admin"])
	}
}

func parseAuditMetadata(t *testing.T, metadata []byte) map[string]interface{} {
	t.Helper()

	var parsed map[string]interface{}
	if err := json.Unmarshal(metadata, &parsed); err != nil {
		t.Fatalf("failed to unmarshal audit metadata: %v", err)
	}

	return parsed
}
