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
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestWatchLogHandlerLogWatch(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchloguser", "watchlog@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test movie")

	handler := NewWatchLogHandler(db, nil)

	watchedAt := time.Date(2025, 1, 15, 20, 0, 0, 0, time.UTC)
	body := bytes.NewBufferString(`{"rating":5,"notes":"Great film!","watched_at":"2025-01-15T20:00:00Z"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID+"/watch-log", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "watchloguser", false))
	w := httptest.NewRecorder()

	handler.LogWatch(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.CreateWatchLogResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.WatchLog.Rating != 5 {
		t.Fatalf("expected rating 5, got %d", response.WatchLog.Rating)
	}

	if response.WatchLog.PostID != uuid.MustParse(postID) {
		t.Fatalf("expected post_id %s, got %s", postID, response.WatchLog.PostID.String())
	}

	if !response.WatchLog.WatchedAt.Equal(watchedAt) {
		t.Fatalf("expected watched_at %s, got %s", watchedAt.Format(time.RFC3339), response.WatchLog.WatchedAt.Format(time.RFC3339))
	}
}

func TestWatchLogHandlerLogWatchRequiresAuth(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewWatchLogHandler(db, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+uuid.New().String()+"/watch-log", bytes.NewBufferString(`{"rating":5,"watched_at":"2025-01-15T20:00:00Z"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.LogWatch(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", w.Code)
	}
}

func TestWatchLogHandlerLogWatchInvalidRating(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchloginvalid", "watchloginvalid@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test movie")

	handler := NewWatchLogHandler(db, nil)
	body := bytes.NewBufferString(`{"rating":6,"watched_at":"2025-01-15T20:00:00Z"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID+"/watch-log", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "watchloginvalid", false))
	w := httptest.NewRecorder()

	handler.LogWatch(w, req)

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

func TestWatchLogHandlerLogWatchNotMovieSection(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchlogrecipe", "watchlogrecipe@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipes", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test recipe")

	handler := NewWatchLogHandler(db, nil)
	body := bytes.NewBufferString(`{"rating":4,"watched_at":"2025-01-15T20:00:00Z"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID+"/watch-log", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "watchlogrecipe", false))
	w := httptest.NewRecorder()

	handler.LogWatch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Code != "POST_NOT_MOVIE_OR_SERIES" {
		t.Fatalf("expected POST_NOT_MOVIE_OR_SERIES, got %s", response.Code)
	}
}

func TestWatchLogPublishesSectionEvent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })
	testutil.CleanupRedis(t)

	redisClient := testutil.GetTestRedis(t)

	userID := testutil.CreateTestUser(t, db, "watchlogeventuser", "watchlogevent@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test movie")

	channel := formatChannel(sectionPrefix, sectionID)
	pubsub := subscribeTestChannel(t, redisClient, channel)

	handler := NewWatchLogHandler(db, redisClient)

	body := bytes.NewBufferString(`{"rating":4,"notes":"Nice","watched_at":"2025-01-15T20:00:00Z"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID+"/watch-log", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "watchlogeventuser", false))
	w := httptest.NewRecorder()

	handler.LogWatch(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	event := receiveEvent(t, pubsub)
	if event.Type != "movie_watched" {
		t.Fatalf("expected movie_watched event, got %s", event.Type)
	}

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		t.Fatalf("failed to marshal event data: %v", err)
	}

	var payload movieWatchedEventData
	if err := json.Unmarshal(dataBytes, &payload); err != nil {
		t.Fatalf("failed to unmarshal movie watched payload: %v", err)
	}

	if payload.PostID.String() != postID {
		t.Fatalf("expected post_id %s, got %s", postID, payload.PostID.String())
	}
	if payload.UserID.String() != userID {
		t.Fatalf("expected user_id %s, got %s", userID, payload.UserID.String())
	}
	if payload.Username != "watchlogeventuser" {
		t.Fatalf("expected username watchlogeventuser, got %s", payload.Username)
	}
	if payload.Rating != 4 {
		t.Fatalf("expected rating 4, got %d", payload.Rating)
	}
}

func TestWatchLogRemovePublishesSectionEvent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })
	testutil.CleanupRedis(t)

	redisClient := testutil.GetTestRedis(t)

	userID := testutil.CreateTestUser(t, db, "watchlogremoveevent", "watchlogremove@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test movie")

	logID := uuid.New()
	if _, err := db.Exec(`
		INSERT INTO watch_logs (id, user_id, post_id, rating, watched_at, created_at)
		VALUES ($1, $2, $3, $4, now(), now())
	`, logID, userID, postID, 5); err != nil {
		t.Fatalf("failed to create watch log: %v", err)
	}

	channel := formatChannel(sectionPrefix, sectionID)
	pubsub := subscribeTestChannel(t, redisClient, channel)

	handler := NewWatchLogHandler(db, redisClient)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/"+postID+"/watch-log", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "watchlogremoveevent", false))
	w := httptest.NewRecorder()

	handler.RemoveWatchLog(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d. Body: %s", w.Code, w.Body.String())
	}

	event := receiveEvent(t, pubsub)
	if event.Type != "movie_watch_removed" {
		t.Fatalf("expected movie_watch_removed event, got %s", event.Type)
	}

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		t.Fatalf("failed to marshal event data: %v", err)
	}

	var payload movieWatchRemovedEventData
	if err := json.Unmarshal(dataBytes, &payload); err != nil {
		t.Fatalf("failed to unmarshal movie watch removed payload: %v", err)
	}

	if payload.PostID.String() != postID {
		t.Fatalf("expected post_id %s, got %s", postID, payload.PostID.String())
	}
	if payload.UserID.String() != userID {
		t.Fatalf("expected user_id %s, got %s", userID, payload.UserID.String())
	}
}

func TestWatchLogHandlerUpdateWatchLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchlogupdateuser", "watchlogupdate@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test movie")

	_, err := db.Exec(`
		INSERT INTO watch_logs (id, user_id, post_id, rating, watched_at, created_at)
		VALUES ($1, $2, $3, $4, now(), now())
	`, uuid.New(), userID, postID, 3)
	if err != nil {
		t.Fatalf("failed to create watch log: %v", err)
	}

	handler := NewWatchLogHandler(db, nil)

	body := bytes.NewBufferString(`{"rating":4,"notes":"Updated notes"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/posts/"+postID+"/watch-log", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "watchlogupdateuser", false))
	w := httptest.NewRecorder()

	handler.UpdateWatchLog(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.UpdateWatchLogResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.WatchLog.Rating != 4 {
		t.Fatalf("expected rating 4, got %d", response.WatchLog.Rating)
	}
}

func TestWatchLogHandlerUpdateWatchLogNoFields(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchlognofields", "watchlognofields@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test movie")

	_, err := db.Exec(`
		INSERT INTO watch_logs (id, user_id, post_id, rating, watched_at, created_at)
		VALUES ($1, $2, $3, $4, now(), now())
	`, uuid.New(), userID, postID, 3)
	if err != nil {
		t.Fatalf("failed to create watch log: %v", err)
	}

	handler := NewWatchLogHandler(db, nil)

	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/posts/"+postID+"/watch-log", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "watchlognofields", false))
	w := httptest.NewRecorder()

	handler.UpdateWatchLog(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Code != "NO_FIELDS_TO_UPDATE" {
		t.Fatalf("expected NO_FIELDS_TO_UPDATE, got %s", response.Code)
	}
}

func TestWatchLogHandlerRemoveWatchLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchlogdeleteuser", "watchlogdelete@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test movie")

	logID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO watch_logs (id, user_id, post_id, rating, watched_at, created_at)
		VALUES ($1, $2, $3, $4, now(), now())
	`, logID, userID, postID, 5)
	if err != nil {
		t.Fatalf("failed to create watch log: %v", err)
	}

	handler := NewWatchLogHandler(db, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/"+postID+"/watch-log", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "watchlogdeleteuser", false))
	w := httptest.NewRecorder()

	handler.RemoveWatchLog(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d. Body: %s", w.Code, w.Body.String())
	}

	var deletedAt sql.NullTime
	if err := db.QueryRow(`SELECT deleted_at FROM watch_logs WHERE id = $1`, logID).Scan(&deletedAt); err != nil {
		t.Fatalf("failed to query deleted watch log: %v", err)
	}
	if !deletedAt.Valid {
		t.Fatalf("expected deleted_at to be set")
	}
}

func TestWatchLogHandlerGetPostWatchLogs(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchloglistuser1", "watchloglist1@test.com", false, true)
	user2ID := testutil.CreateTestUser(t, db, "watchloglistuser2", "watchloglist2@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test movie")

	_, err := db.Exec(`
		INSERT INTO watch_logs (id, user_id, post_id, rating, watched_at, created_at)
		VALUES ($1, $2, $3, $4, now(), now()), ($5, $6, $7, $8, now(), now())
	`,
		uuid.New(), userID, postID, 5,
		uuid.New(), user2ID, postID, 4,
	)
	if err != nil {
		t.Fatalf("failed to create watch logs: %v", err)
	}

	handler := NewWatchLogHandler(db, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/"+postID+"/watch-logs", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "watchloglistuser1", false))
	w := httptest.NewRecorder()

	handler.GetPostWatchLogs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.PostWatchLogsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.WatchCount != 2 {
		t.Fatalf("expected watch_count 2, got %d", response.WatchCount)
	}

	if response.AvgRating == nil || *response.AvgRating < 4.4 || *response.AvgRating > 4.6 {
		t.Fatalf("expected avg_rating around 4.5, got %v", response.AvgRating)
	}

	if len(response.Logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(response.Logs))
	}

	if !response.ViewerWatched || response.ViewerRating == nil {
		t.Fatalf("expected viewer watch data")
	}

	if *response.ViewerRating != 5 {
		t.Fatalf("expected viewer rating 5, got %d", *response.ViewerRating)
	}
}

func TestWatchLogHandlerGetMyWatchLogsPagination(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "watchlogmeuser", "watchlogme@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Movies", "movie")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test movie")
	post2ID := testutil.CreateTestPost(t, db, userID, sectionID, "Another movie")

	log1ID := uuid.New()
	log2ID := uuid.New()
	if _, err := db.Exec(`
		INSERT INTO watch_logs (id, user_id, post_id, rating, watched_at, created_at)
		VALUES ($1, $2, $3, $4, $5, now()), ($6, $7, $8, $9, $10, now())
	`,
		log1ID, userID, postID, 5, time.Date(2025, 1, 10, 20, 0, 0, 0, time.UTC),
		log2ID, userID, post2ID, 4, time.Date(2025, 1, 11, 20, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatalf("failed to create watch logs: %v", err)
	}

	handler := NewWatchLogHandler(db, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/watch-logs?limit=1", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "watchlogmeuser", false))
	w := httptest.NewRecorder()

	handler.GetMyWatchLogs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.ListWatchLogsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.WatchLogs) != 1 {
		t.Fatalf("expected 1 watch log, got %d", len(response.WatchLogs))
	}
	if response.NextCursor == nil {
		t.Fatalf("expected next_cursor to be set")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/me/watch-logs?limit=1&cursor="+*response.NextCursor, nil)
	req2 = req2.WithContext(createTestUserContext(req2.Context(), uuid.MustParse(userID), "watchlogmeuser", false))
	w2 := httptest.NewRecorder()

	handler.GetMyWatchLogs(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w2.Code, w2.Body.String())
	}

	var response2 models.ListWatchLogsResponse
	if err := json.NewDecoder(w2.Body).Decode(&response2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response2.WatchLogs) != 1 {
		t.Fatalf("expected 1 watch log, got %d", len(response2.WatchLogs))
	}
	if response2.NextCursor != nil {
		t.Fatalf("expected next_cursor to be nil")
	}
}
