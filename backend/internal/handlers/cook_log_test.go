package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestCookLogHandlerLogCook(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "cookloguser", "cooklog@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipe Section", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test recipe")

	handler := NewCookLogHandler(db, nil)

	body := bytes.NewBufferString(`{"rating":5,"notes":"Added extra garlic"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID+"/cook-log", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "cookloguser", false))
	w := httptest.NewRecorder()

	handler.LogCook(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.CreateCookLogResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.CookLog.Rating != 5 {
		t.Fatalf("expected rating 5, got %d", response.CookLog.Rating)
	}

	if response.CookLog.PostID != uuid.MustParse(postID) {
		t.Fatalf("expected post_id %s, got %s", postID, response.CookLog.PostID.String())
	}
}

func TestCookLogPublishesSectionEvent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })
	testutil.CleanupRedis(t)

	redisClient := testutil.GetTestRedis(t)

	userID := testutil.CreateTestUser(t, db, "cooklogeventuser", "cooklogevent@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipe Section", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test recipe")

	channel := formatChannel(sectionPrefix, sectionID)
	pubsub := subscribeTestChannel(t, redisClient, channel)

	handler := NewCookLogHandler(db, redisClient)

	body := bytes.NewBufferString(`{"rating":4,"notes":"Nice"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/"+postID+"/cook-log", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "cooklogeventuser", false))
	w := httptest.NewRecorder()

	handler.LogCook(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	event := receiveEvent(t, pubsub)
	if event.Type != "recipe_cooked" {
		t.Fatalf("expected recipe_cooked event, got %s", event.Type)
	}

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		t.Fatalf("failed to marshal event data: %v", err)
	}

	var payload recipeCookedEventData
	if err := json.Unmarshal(dataBytes, &payload); err != nil {
		t.Fatalf("failed to unmarshal recipe cooked payload: %v", err)
	}

	if payload.PostID.String() != postID {
		t.Fatalf("expected post_id %s, got %s", postID, payload.PostID.String())
	}
	if payload.UserID.String() != userID {
		t.Fatalf("expected user_id %s, got %s", userID, payload.UserID.String())
	}
	if payload.Username != "cooklogeventuser" {
		t.Fatalf("expected username cooklogeventuser, got %s", payload.Username)
	}
	if payload.Rating != 4 {
		t.Fatalf("expected rating 4, got %d", payload.Rating)
	}
}

func TestCookLogRemovePublishesSectionEvent(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })
	testutil.CleanupRedis(t)

	redisClient := testutil.GetTestRedis(t)

	userID := testutil.CreateTestUser(t, db, "cooklogremoveevent", "cooklogremove@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipe Section", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test recipe")

	logID := uuid.New()
	if _, err := db.Exec(`
		INSERT INTO cook_logs (id, user_id, post_id, rating, created_at)
		VALUES ($1, $2, $3, $4, now())
	`, logID, userID, postID, 5); err != nil {
		t.Fatalf("failed to create cook log: %v", err)
	}

	channel := formatChannel(sectionPrefix, sectionID)
	pubsub := subscribeTestChannel(t, redisClient, channel)

	handler := NewCookLogHandler(db, redisClient)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/"+postID+"/cook-log", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "cooklogremoveevent", false))
	w := httptest.NewRecorder()

	handler.RemoveCookLog(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d. Body: %s", w.Code, w.Body.String())
	}

	event := receiveEvent(t, pubsub)
	if event.Type != "recipe_cook_removed" {
		t.Fatalf("expected recipe_cook_removed event, got %s", event.Type)
	}

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		t.Fatalf("failed to marshal event data: %v", err)
	}

	var payload recipeCookRemovedEventData
	if err := json.Unmarshal(dataBytes, &payload); err != nil {
		t.Fatalf("failed to unmarshal recipe cook removed payload: %v", err)
	}

	if payload.PostID.String() != postID {
		t.Fatalf("expected post_id %s, got %s", postID, payload.PostID.String())
	}
	if payload.UserID.String() != userID {
		t.Fatalf("expected user_id %s, got %s", userID, payload.UserID.String())
	}
}

func TestCookLogHandlerUpdateCookLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "cooklogupdateuser", "cooklogupdate@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipe Section", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test recipe")

	_, err := db.Exec(`
		INSERT INTO cook_logs (id, user_id, post_id, rating, created_at)
		VALUES ($1, $2, $3, $4, now())
	`, uuid.New(), userID, postID, 3)
	if err != nil {
		t.Fatalf("failed to create cook log: %v", err)
	}

	handler := NewCookLogHandler(db, nil)

	body := bytes.NewBufferString(`{"rating":4,"notes":"Updated notes"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/posts/"+postID+"/cook-log", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "cooklogupdateuser", false))
	w := httptest.NewRecorder()

	handler.UpdateCookLog(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.UpdateCookLogResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.CookLog.Rating != 4 {
		t.Fatalf("expected rating 4, got %d", response.CookLog.Rating)
	}
}

func TestCookLogHandlerRemoveCookLog(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "cooklogdeleteuser", "cooklogdelete@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipe Section", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test recipe")

	logID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO cook_logs (id, user_id, post_id, rating, created_at)
		VALUES ($1, $2, $3, $4, now())
	`, logID, userID, postID, 5)
	if err != nil {
		t.Fatalf("failed to create cook log: %v", err)
	}

	handler := NewCookLogHandler(db, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/posts/"+postID+"/cook-log", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "cooklogdeleteuser", false))
	w := httptest.NewRecorder()

	handler.RemoveCookLog(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d. Body: %s", w.Code, w.Body.String())
	}

	var deletedAt sql.NullTime
	if err := db.QueryRow(`SELECT deleted_at FROM cook_logs WHERE id = $1`, logID).Scan(&deletedAt); err != nil {
		t.Fatalf("failed to query deleted cook log: %v", err)
	}
	if !deletedAt.Valid {
		t.Fatalf("expected deleted_at to be set")
	}
}

func TestCookLogHandlerGetPostCookLogs(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "cookloglistuser1", "cookloglist1@test.com", false, true)
	user2ID := testutil.CreateTestUser(t, db, "cookloglistuser2", "cookloglist2@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipe Section", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test recipe")

	_, err := db.Exec(`
		INSERT INTO cook_logs (id, user_id, post_id, rating, created_at)
		VALUES ($1, $2, $3, $4, now()), ($5, $6, $7, $8, now())
	`,
		uuid.New(), userID, postID, 5,
		uuid.New(), user2ID, postID, 4,
	)
	if err != nil {
		t.Fatalf("failed to create cook logs: %v", err)
	}

	handler := NewCookLogHandler(db, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/"+postID+"/cook-logs", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "cookloglistuser1", false))
	w := httptest.NewRecorder()

	handler.GetPostCookLogs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.PostCookInfo
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.CookCount != 2 {
		t.Fatalf("expected cook_count 2, got %d", response.CookCount)
	}

	if response.AvgRating == nil || *response.AvgRating < 4.4 || *response.AvgRating > 4.6 {
		t.Fatalf("expected avg_rating around 4.5, got %v", response.AvgRating)
	}

	if len(response.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(response.Users))
	}

	if !response.ViewerCooked || response.ViewerCookLog == nil {
		t.Fatalf("expected viewer cook log")
	}

	if response.ViewerCookLog.Rating != 5 {
		t.Fatalf("expected viewer rating 5, got %d", response.ViewerCookLog.Rating)
	}
}

func TestCookLogHandlerGetMyCookLogs(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := testutil.CreateTestUser(t, db, "cooklogmeuser", "cooklogme@test.com", false, true)
	sectionID := testutil.CreateTestSection(t, db, "Recipe Section", "recipe")
	postID := testutil.CreateTestPost(t, db, userID, sectionID, "Test recipe")
	post2ID := testutil.CreateTestPost(t, db, userID, sectionID, "Another recipe")

	_, err := db.Exec(`
		INSERT INTO cook_logs (id, user_id, post_id, rating, created_at)
		VALUES ($1, $2, $3, $4, now()), ($5, $6, $7, $8, now())
	`,
		uuid.New(), userID, postID, 5,
		uuid.New(), userID, post2ID, 4,
	)
	if err != nil {
		t.Fatalf("failed to create cook logs: %v", err)
	}

	handler := NewCookLogHandler(db, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/cook-logs", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.MustParse(userID), "cooklogmeuser", false))
	w := httptest.NewRecorder()

	handler.GetMyCookLogs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response models.ListCookLogsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.CookLogs) != 2 {
		t.Fatalf("expected 2 cook logs, got %d", len(response.CookLogs))
	}

	if response.Meta.HasMore {
		t.Fatalf("expected has_more to be false")
	}
}
