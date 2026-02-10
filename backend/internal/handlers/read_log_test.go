package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestReadLogHandlerLogRead(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "readloghandleruser", "readloghandler@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Book post")

	handler := NewReadLogHandler(services.NewReadLogService(db))

	body := bytes.NewBufferString(`{"rating":4}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID+"/read", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "readloghandleruser", false))
	w := httptest.NewRecorder()

	handler.LogRead(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.CreateReadLogResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.ReadLog.PostID != uuid.MustParse(postID) {
		t.Fatalf("expected post_id %s, got %s", postID, response.ReadLog.PostID.String())
	}
	if response.ReadLog.Rating == nil || *response.ReadLog.Rating != 4 {
		t.Fatalf("expected rating 4, got %v", response.ReadLog.Rating)
	}
}

func TestReadLogHandlerLogReadRequiresAuth(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewReadLogHandler(services.NewReadLogService(db))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+uuid.New().String()+"/read", bytes.NewBufferString(`{"rating":3}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.LogRead(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", w.Code)
	}
}

func TestReadLogHandlerLogReadInvalidRating(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "readloginvalid", "readloginvalid@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Book post")

	handler := NewReadLogHandler(services.NewReadLogService(db))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID+"/read", bytes.NewBufferString(`{"rating":6}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "readloginvalid", false))
	w := httptest.NewRecorder()

	handler.LogRead(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Code != "INVALID_RATING" {
		t.Fatalf("expected INVALID_RATING, got %s", response.Code)
	}
}

func TestReadLogHandlerLogReadPostNotFound(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "readlogpostmissing", "readlogpostmissing@test.com", false, true)
	handler := NewReadLogHandler(services.NewReadLogService(db))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+uuid.New().String()+"/read", bytes.NewBufferString(`{"rating":3}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "readlogpostmissing", false))
	w := httptest.NewRecorder()

	handler.LogRead(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Code != "POST_NOT_FOUND" {
		t.Fatalf("expected POST_NOT_FOUND, got %s", response.Code)
	}
}

func TestReadLogHandlerRemoveReadLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "readlogremove", "readlogremove@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Book post")

	readLogID := uuid.New()
	if _, err := db.Exec(`
		INSERT INTO read_logs (id, user_id, post_id, rating, created_at)
		VALUES ($1, $2, $3, $4, now())
	`, readLogID, userID, postID, 5); err != nil {
		t.Fatalf("failed to create read log: %v", err)
	}

	handler := NewReadLogHandler(services.NewReadLogService(db))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/"+postID+"/read", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "readlogremove", false))
	w := httptest.NewRecorder()

	handler.RemoveReadLog(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d. Body: %s", w.Code, w.Body.String())
	}

	var deletedAt sql.NullTime
	if err := db.QueryRow(`SELECT deleted_at FROM read_logs WHERE id = $1`, readLogID).Scan(&deletedAt); err != nil {
		t.Fatalf("failed to query deleted read log: %v", err)
	}
	if !deletedAt.Valid {
		t.Fatal("expected deleted_at to be set")
	}
}

func TestReadLogHandlerRemoveReadLogNotFound(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "readlogremove404", "readlogremove404@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Book post")

	handler := NewReadLogHandler(services.NewReadLogService(db))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/"+postID+"/read", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "readlogremove404", false))
	w := httptest.NewRecorder()

	handler.RemoveReadLog(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Code != "READ_LOG_NOT_FOUND" {
		t.Fatalf("expected READ_LOG_NOT_FOUND, got %s", response.Code)
	}
}

func TestReadLogHandlerUpdateReadLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "readlogupdate", "readlogupdate@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Book post")

	if _, err := db.Exec(`
		INSERT INTO read_logs (id, user_id, post_id, rating, created_at)
		VALUES ($1, $2, $3, $4, now())
	`, uuid.New(), userID, postID, 2); err != nil {
		t.Fatalf("failed to create read log: %v", err)
	}

	handler := NewReadLogHandler(services.NewReadLogService(db))

	req := httptest.NewRequest(http.MethodPut, "/api/v1/posts/"+postID+"/read", bytes.NewBufferString(`{"rating":5}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "readlogupdate", false))
	w := httptest.NewRecorder()

	handler.UpdateReadLog(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.UpdateReadLogResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.ReadLog.Rating == nil || *response.ReadLog.Rating != 5 {
		t.Fatalf("expected rating 5, got %v", response.ReadLog.Rating)
	}
}

func TestReadLogHandlerUpdateReadLogInvalidRating(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "readlogupdateinvalid", "readlogupdateinvalid@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Book post")

	handler := NewReadLogHandler(services.NewReadLogService(db))

	req := httptest.NewRequest(http.MethodPut, "/api/v1/posts/"+postID+"/read", bytes.NewBufferString(`{"rating":0}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "readlogupdateinvalid", false))
	w := httptest.NewRecorder()

	handler.UpdateReadLog(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Code != "INVALID_RATING" {
		t.Fatalf("expected INVALID_RATING, got %s", response.Code)
	}
}

func TestReadLogHandlerUpdateReadLogNotFound(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "readlogupdate404", "readlogupdate404@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Book post")

	handler := NewReadLogHandler(services.NewReadLogService(db))

	req := httptest.NewRequest(http.MethodPut, "/api/v1/posts/"+postID+"/read", bytes.NewBufferString(`{"rating":3}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "readlogupdate404", false))
	w := httptest.NewRecorder()

	handler.UpdateReadLog(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Code != "READ_LOG_NOT_FOUND" {
		t.Fatalf("expected READ_LOG_NOT_FOUND, got %s", response.Code)
	}
}

func TestReadLogHandlerGetPostReadLogs(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	viewerID := testutil.CreateTestUser(t, db, "readlogviewer", "readlogviewer@test.com", false, true)
	otherUserID := testutil.CreateTestUser(t, db, "readlogviewer2", "readlogviewer2@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postID := testutil.CreateTestPost(t, db, viewerID, sectionID, "Book post")

	if _, err := db.Exec(`
		INSERT INTO read_logs (id, user_id, post_id, rating, created_at)
		VALUES ($1, $2, $3, $4, now()), ($5, $6, $7, $8, now())
	`,
		uuid.New(), viewerID, postID, 5,
		uuid.New(), otherUserID, postID, 3,
	); err != nil {
		t.Fatalf("failed to create read logs: %v", err)
	}

	handler := NewReadLogHandler(services.NewReadLogService(db))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/"+postID+"/read", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(viewerID), "readlogviewer", false))
	w := httptest.NewRecorder()

	handler.GetPostReadLogs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.PostReadLogsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.ReadCount != 2 {
		t.Fatalf("expected read_count 2, got %d", response.ReadCount)
	}
	if response.AverageRating < 3.9 || response.AverageRating > 4.1 {
		t.Fatalf("expected average_rating around 4.0, got %f", response.AverageRating)
	}
	if !response.ViewerRead {
		t.Fatal("expected viewer_read true")
	}
	if response.ViewerRating == nil || *response.ViewerRating != 5 {
		t.Fatalf("expected viewer_rating 5, got %v", response.ViewerRating)
	}
	if len(response.Readers) != 2 {
		t.Fatalf("expected 2 readers, got %d", len(response.Readers))
	}
}

func TestReadLogHandlerGetPostReadLogsNotFound(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "readloglist404", "readloglist404@test.com", false, true)
	handler := NewReadLogHandler(services.NewReadLogService(db))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/"+uuid.New().String()+"/read", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "readloglist404", false))
	w := httptest.NewRecorder()

	handler.GetPostReadLogs(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Code != "POST_NOT_FOUND" {
		t.Fatalf("expected POST_NOT_FOUND, got %s", response.Code)
	}
}

func TestReadLogHandlerGetReadHistory(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "readhistoryhandler", "readhistoryhandler@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Books", "book")
	postIDOne := testutil.CreateTestPost(t, db, userID, sectionID, "History one")
	postIDTwo := testutil.CreateTestPost(t, db, userID, sectionID, "History two")

	logOneID := uuid.New()
	logTwoID := uuid.New()
	firstCreatedAt := time.Now().Add(-2 * time.Hour)
	secondCreatedAt := time.Now().Add(-1 * time.Hour)
	if _, err := db.Exec(`
		INSERT INTO read_logs (id, user_id, post_id, rating, created_at)
		VALUES ($1, $2, $3, $4, $5), ($6, $7, $8, $9, $10)
	`,
		logOneID, userID, postIDOne, 2, firstCreatedAt,
		logTwoID, userID, postIDTwo, 5, secondCreatedAt,
	); err != nil {
		t.Fatalf("failed to create read logs: %v", err)
	}

	handler := NewReadLogHandler(services.NewReadLogService(db))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/read-history?limit=1", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "readhistoryhandler", false))
	w := httptest.NewRecorder()

	handler.GetReadHistory(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.ListReadHistoryResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(response.ReadLogs) != 1 {
		t.Fatalf("expected 1 read log, got %d", len(response.ReadLogs))
	}
	if response.NextCursor == nil {
		t.Fatal("expected next_cursor to be set")
	}
	if response.ReadLogs[0].PostID != uuid.MustParse(postIDTwo) {
		t.Fatalf("expected newest post_id %s, got %s", postIDTwo, response.ReadLogs[0].PostID.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/read-history?limit=1&cursor="+*response.NextCursor, nil)
	req2 = req2.WithContext(createTestUserContext(req2.Context(), uuid.MustParse(userID), "readhistoryhandler", false))
	w2 := httptest.NewRecorder()

	handler.GetReadHistory(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w2.Code, w2.Body.String())
	}

	var response2 models.ListReadHistoryResponse
	if err := json.NewDecoder(w2.Body).Decode(&response2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(response2.ReadLogs) != 1 {
		t.Fatalf("expected 1 read log on second page, got %d", len(response2.ReadLogs))
	}
	if response2.NextCursor != nil {
		t.Fatal("expected next_cursor to be nil on second page")
	}
	if response2.ReadLogs[0].PostID != uuid.MustParse(postIDOne) {
		t.Fatalf("expected older post_id %s, got %s", postIDOne, response2.ReadLogs[0].PostID.String())
	}
}

func TestReadLogHandlerGetReadHistoryInvalidCursor(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "readhistorycursor", "readhistorycursor@test.com", false, true)
	handler := NewReadLogHandler(services.NewReadLogService(db))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/read-history?cursor=bad-cursor", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "readhistorycursor", false))
	w := httptest.NewRecorder()

	handler.GetReadHistory(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Code != "INVALID_CURSOR" {
		t.Fatalf("expected INVALID_CURSOR, got %s", response.Code)
	}
}

func TestReadLogHandlerGetReadHistoryRequiresAuth(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewReadLogHandler(services.NewReadLogService(db))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/read-history", nil)
	w := httptest.NewRecorder()

	handler.GetReadHistory(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", w.Code)
	}
}
