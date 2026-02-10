package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestBookQuoteHandlerCreateQuote(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquotehandlercreate", "bookquotehandlercreate@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book quote post")

	handler := NewBookQuoteHandler(services.NewBookQuoteService(db))

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/posts/"+postID+"/quotes",
		bytes.NewBufferString(`{"quote_text":"A quote worth saving","page_number":12}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), userID, "bookquotehandlercreate", false))
	w := httptest.NewRecorder()

	handler.CreateQuote(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var response models.BookQuoteResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Quote.PostID != uuid.MustParse(postID) {
		t.Fatalf("expected post id %s, got %s", postID, response.Quote.PostID.String())
	}
	if response.Quote.UserID != userID {
		t.Fatalf("expected user id %s, got %s", userID.String(), response.Quote.UserID.String())
	}
	if response.Quote.PageNumber == nil || *response.Quote.PageNumber != 12 {
		t.Fatalf("expected page_number 12, got %v", response.Quote.PageNumber)
	}
}

func TestBookQuoteHandlerCreateQuoteRequiresAuth(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewBookQuoteHandler(services.NewBookQuoteService(db))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+uuid.New().String()+"/quotes", bytes.NewBufferString(`{"quote_text":"No auth"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateQuote(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestBookQuoteHandlerCreateQuoteInvalidInput(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquotehandlerinvalid", "bookquotehandlerinvalid@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book quote post")

	handler := NewBookQuoteHandler(services.NewBookQuoteService(db))

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/posts/"+postID+"/quotes",
		bytes.NewBufferString(`{"quote_text":"   "}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), userID, "bookquotehandlerinvalid", false))
	w := httptest.NewRecorder()

	handler.CreateQuote(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if response.Code != "QUOTE_TEXT_REQUIRED" {
		t.Fatalf("expected code QUOTE_TEXT_REQUIRED, got %s", response.Code)
	}
}

func TestBookQuoteHandlerGetPostQuotesPagination(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquotehandlerlist", "bookquotehandlerlist@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book quote post"))

	service := services.NewBookQuoteService(db)
	oldQuote, err := service.CreateQuote(reqContext(), userID, postID, models.CreateBookQuoteRequest{QuoteText: "Old quote"})
	if err != nil {
		t.Fatalf("CreateQuote oldQuote failed: %v", err)
	}
	newQuote, err := service.CreateQuote(reqContext(), userID, postID, models.CreateBookQuoteRequest{QuoteText: "New quote"})
	if err != nil {
		t.Fatalf("CreateQuote newQuote failed: %v", err)
	}

	now := time.Now().UTC()
	if _, err := db.Exec(`UPDATE book_quotes SET created_at = $1 WHERE id = $2`, now.Add(-2*time.Hour), oldQuote.ID); err != nil {
		t.Fatalf("failed to set old quote created_at: %v", err)
	}
	if _, err := db.Exec(`UPDATE book_quotes SET created_at = $1 WHERE id = $2`, now.Add(-1*time.Hour), newQuote.ID); err != nil {
		t.Fatalf("failed to set new quote created_at: %v", err)
	}

	handler := NewBookQuoteHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/"+postID.String()+"/quotes?limit=1", nil)
	req = req.WithContext(createTestUserContext(req.Context(), userID, "bookquotehandlerlist", false))
	w := httptest.NewRecorder()

	handler.GetPostQuotes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var firstPage models.BookQuotesListResponse
	if err := json.NewDecoder(w.Body).Decode(&firstPage); err != nil {
		t.Fatalf("failed to decode first page: %v", err)
	}
	if len(firstPage.Quotes) != 1 {
		t.Fatalf("expected 1 quote on first page, got %d", len(firstPage.Quotes))
	}
	if !firstPage.HasMore {
		t.Fatal("expected has_more=true on first page")
	}
	if firstPage.NextCursor == nil {
		t.Fatal("expected next_cursor on first page")
	}
	if firstPage.Quotes[0].QuoteText != "New quote" {
		t.Fatalf("expected newest quote first, got %q", firstPage.Quotes[0].QuoteText)
	}

	req2 := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/posts/"+postID.String()+"/quotes?limit=1&cursor="+url.QueryEscape(*firstPage.NextCursor),
		nil,
	)
	req2 = req2.WithContext(createTestUserContext(req2.Context(), userID, "bookquotehandlerlist", false))
	w2 := httptest.NewRecorder()

	handler.GetPostQuotes(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w2.Code, w2.Body.String())
	}

	var secondPage models.BookQuotesListResponse
	if err := json.NewDecoder(w2.Body).Decode(&secondPage); err != nil {
		t.Fatalf("failed to decode second page: %v", err)
	}
	if len(secondPage.Quotes) != 1 {
		t.Fatalf("expected 1 quote on second page, got %d", len(secondPage.Quotes))
	}
	if secondPage.HasMore {
		t.Fatal("expected has_more=false on second page")
	}
	if secondPage.NextCursor != nil {
		t.Fatal("expected next_cursor=nil on final page")
	}
	if secondPage.Quotes[0].QuoteText != "Old quote" {
		t.Fatalf("expected oldest quote second, got %q", secondPage.Quotes[0].QuoteText)
	}
}

func TestBookQuoteHandlerGetPostQuotesInvalidPaginationParams(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquotehandlerbadlimit", "bookquotehandlerbadlimit@test.com", false, true))
	postID := uuid.New()

	handler := NewBookQuoteHandler(services.NewBookQuoteService(db))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/"+postID.String()+"/quotes?limit=not-a-number", nil)
	req = req.WithContext(createTestUserContext(req.Context(), userID, "bookquotehandlerbadlimit", false))
	w := httptest.NewRecorder()

	handler.GetPostQuotes(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Code != "INVALID_LIMIT" {
		t.Fatalf("expected INVALID_LIMIT, got %s", response.Code)
	}
}

func TestBookQuoteHandlerUpdateQuote(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquotehandlerupdate", "bookquotehandlerupdate@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book quote post"))

	service := services.NewBookQuoteService(db)
	created, err := service.CreateQuote(reqContext(), userID, postID, models.CreateBookQuoteRequest{QuoteText: "Before"})
	if err != nil {
		t.Fatalf("CreateQuote failed: %v", err)
	}

	handler := NewBookQuoteHandler(service)

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/quotes/"+created.ID.String(),
		bytes.NewBufferString(`{"quote_text":"After"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), userID, "bookquotehandlerupdate", false))
	w := httptest.NewRecorder()

	handler.UpdateQuote(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.BookQuoteResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Quote.QuoteText != "After" {
		t.Fatalf("expected updated quote text, got %q", response.Quote.QuoteText)
	}
}

func TestBookQuoteHandlerUpdateQuoteOwnershipValidation(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	ownerID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquotehandlerowner", "bookquotehandlerowner@test.com", false, true))
	otherID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquotehandlerother", "bookquotehandlerother@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, ownerID.String(), sectionID, "Book quote post"))

	service := services.NewBookQuoteService(db)
	created, err := service.CreateQuote(reqContext(), ownerID, postID, models.CreateBookQuoteRequest{QuoteText: "Owner quote"})
	if err != nil {
		t.Fatalf("CreateQuote failed: %v", err)
	}

	handler := NewBookQuoteHandler(service)

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/quotes/"+created.ID.String(),
		bytes.NewBufferString(`{"quote_text":"Should fail"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), otherID, "bookquotehandlerother", false))
	w := httptest.NewRecorder()

	handler.UpdateQuote(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusForbidden, w.Code, w.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Code != "FORBIDDEN" {
		t.Fatalf("expected FORBIDDEN, got %s", response.Code)
	}
}

func TestBookQuoteHandlerDeleteQuote(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquotehandlerdelete", "bookquotehandlerdelete@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, userID.String(), sectionID, "Book quote post"))

	service := services.NewBookQuoteService(db)
	created, err := service.CreateQuote(reqContext(), userID, postID, models.CreateBookQuoteRequest{QuoteText: "Delete me"})
	if err != nil {
		t.Fatalf("CreateQuote failed: %v", err)
	}

	handler := NewBookQuoteHandler(service)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/quotes/"+created.ID.String(), nil)
	req = req.WithContext(createTestUserContext(req.Context(), userID, "bookquotehandlerdelete", false))
	w := httptest.NewRecorder()

	handler.DeleteQuote(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusNoContent, w.Code, w.Body.String())
	}

	var deletedAt sql.NullTime
	if err := db.QueryRow(`SELECT deleted_at FROM book_quotes WHERE id = $1`, created.ID).Scan(&deletedAt); err != nil {
		t.Fatalf("failed to query deleted quote: %v", err)
	}
	if !deletedAt.Valid {
		t.Fatal("expected deleted_at to be set")
	}
}

func TestBookQuoteHandlerDeleteQuoteOwnershipValidation(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	ownerID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquotehandlerowner2", "bookquotehandlerowner2@test.com", false, true))
	otherID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquotehandlerother2", "bookquotehandlerother2@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := uuid.MustParse(testutil.CreateTestPost(t, db, ownerID.String(), sectionID, "Book quote post"))

	service := services.NewBookQuoteService(db)
	created, err := service.CreateQuote(reqContext(), ownerID, postID, models.CreateBookQuoteRequest{QuoteText: "Owner quote"})
	if err != nil {
		t.Fatalf("CreateQuote failed: %v", err)
	}

	handler := NewBookQuoteHandler(service)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/quotes/"+created.ID.String(), nil)
	req = req.WithContext(createTestUserContext(req.Context(), otherID, "bookquotehandlerother2", false))
	w := httptest.NewRecorder()

	handler.DeleteQuote(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusForbidden, w.Code, w.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Code != "FORBIDDEN" {
		t.Fatalf("expected FORBIDDEN, got %s", response.Code)
	}
}

func TestBookQuoteHandlerGetUserQuotes(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	viewerID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquotehandlerviewer", "bookquotehandlerviewer@test.com", false, true))
	targetID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquotehandlertarget", "bookquotehandlertarget@test.com", false, true))
	otherID := uuid.MustParse(testutil.CreateTestUser(t, db, "bookquotehandlerother3", "bookquotehandlerother3@test.com", false, true))
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")

	targetPostID := uuid.MustParse(testutil.CreateTestPost(t, db, targetID.String(), sectionID, "Target post"))
	otherPostID := uuid.MustParse(testutil.CreateTestPost(t, db, otherID.String(), sectionID, "Other post"))

	service := services.NewBookQuoteService(db)
	targetOld, err := service.CreateQuote(reqContext(), targetID, targetPostID, models.CreateBookQuoteRequest{QuoteText: "Target old"})
	if err != nil {
		t.Fatalf("CreateQuote targetOld failed: %v", err)
	}
	targetNew, err := service.CreateQuote(reqContext(), targetID, targetPostID, models.CreateBookQuoteRequest{QuoteText: "Target new"})
	if err != nil {
		t.Fatalf("CreateQuote targetNew failed: %v", err)
	}
	if _, err := service.CreateQuote(reqContext(), otherID, otherPostID, models.CreateBookQuoteRequest{QuoteText: "Other user quote"}); err != nil {
		t.Fatalf("CreateQuote other user failed: %v", err)
	}

	now := time.Now().UTC()
	if _, err := db.Exec(`UPDATE book_quotes SET created_at = $1 WHERE id = $2`, now.Add(-2*time.Hour), targetOld.ID); err != nil {
		t.Fatalf("failed to set targetOld created_at: %v", err)
	}
	if _, err := db.Exec(`UPDATE book_quotes SET created_at = $1 WHERE id = $2`, now.Add(-1*time.Hour), targetNew.ID); err != nil {
		t.Fatalf("failed to set targetNew created_at: %v", err)
	}

	handler := NewBookQuoteHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+targetID.String()+"/quotes?limit=1", nil)
	req = req.WithContext(createTestUserContext(req.Context(), viewerID, "bookquotehandlerviewer", false))
	w := httptest.NewRecorder()

	handler.GetUserQuotes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var firstPage models.BookQuotesListResponse
	if err := json.NewDecoder(w.Body).Decode(&firstPage); err != nil {
		t.Fatalf("failed to decode first page: %v", err)
	}
	if len(firstPage.Quotes) != 1 {
		t.Fatalf("expected 1 quote on first page, got %d", len(firstPage.Quotes))
	}
	if firstPage.Quotes[0].UserID != targetID {
		t.Fatalf("expected quote from target user, got %s", firstPage.Quotes[0].UserID.String())
	}
	if !firstPage.HasMore {
		t.Fatal("expected has_more=true on first page")
	}
	if firstPage.NextCursor == nil {
		t.Fatal("expected next_cursor on first page")
	}

	req2 := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/users/"+targetID.String()+"/quotes?limit=1&cursor="+url.QueryEscape(*firstPage.NextCursor),
		nil,
	)
	req2 = req2.WithContext(createTestUserContext(req2.Context(), viewerID, "bookquotehandlerviewer", false))
	w2 := httptest.NewRecorder()

	handler.GetUserQuotes(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w2.Code, w2.Body.String())
	}

	var secondPage models.BookQuotesListResponse
	if err := json.NewDecoder(w2.Body).Decode(&secondPage); err != nil {
		t.Fatalf("failed to decode second page: %v", err)
	}
	if len(secondPage.Quotes) != 1 {
		t.Fatalf("expected 1 quote on second page, got %d", len(secondPage.Quotes))
	}
	if secondPage.Quotes[0].UserID != targetID {
		t.Fatalf("expected quote from target user, got %s", secondPage.Quotes[0].UserID.String())
	}
	if secondPage.HasMore {
		t.Fatal("expected has_more=false on second page")
	}
}

func TestBookQuoteHandlerUnauthorizedProtectedEndpoints(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewBookQuoteHandler(services.NewBookQuoteService(db))
	postID := uuid.New().String()
	quoteID := uuid.New().String()
	userID := uuid.New().String()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		call   func(http.ResponseWriter, *http.Request)
	}{
		{
			name:   "get post quotes",
			method: http.MethodGet,
			path:   "/api/v1/posts/" + postID + "/quotes",
			call:   handler.GetPostQuotes,
		},
		{
			name:   "update quote",
			method: http.MethodPut,
			path:   "/api/v1/quotes/" + quoteID,
			body:   `{"quote_text":"updated"}`,
			call:   handler.UpdateQuote,
		},
		{
			name:   "delete quote",
			method: http.MethodDelete,
			path:   "/api/v1/quotes/" + quoteID,
			call:   handler.DeleteQuote,
		},
		{
			name:   "get user quotes",
			method: http.MethodGet,
			path:   "/api/v1/users/" + userID + "/quotes",
			call:   handler.GetUserQuotes,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := bytes.NewBufferString(tc.body)
			req := httptest.NewRequest(tc.method, tc.path, body)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()

			tc.call(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, w.Code, w.Body.String())
			}
		})
	}
}
