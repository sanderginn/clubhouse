package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/testutil"
)

func insertTestNotification(t *testing.T, db *sql.DB, id uuid.UUID, userID uuid.UUID, createdAt time.Time, readAt *time.Time) {
	t.Helper()

	var readAtValue interface{} = nil
	if readAt != nil {
		readAtValue = *readAt
	}

	_, err := db.Exec(`
		INSERT INTO notifications (id, user_id, type, read_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, id, userID, "mention", readAtValue, createdAt)
	if err != nil {
		t.Fatalf("failed to insert notification: %v", err)
	}
}

func TestGetNotificationsSuccessPagination(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "notifuser", "notifuser@test.com", false, true))
	handler := NewNotificationHandler(db, nil)

	now := time.Now().UTC()
	notificationID1 := uuid.New()
	notificationID2 := uuid.New()
	notificationID3 := uuid.New()

	readAt := now.Add(-90 * time.Minute)
	insertTestNotification(t, db, notificationID1, userID, now.Add(-2*time.Hour), &readAt)
	insertTestNotification(t, db, notificationID2, userID, now.Add(-1*time.Hour), nil)
	insertTestNotification(t, db, notificationID3, userID, now.Add(-30*time.Minute), nil)

	req := httptest.NewRequest("GET", "/api/v1/notifications?limit=2", nil)
	req = req.WithContext(createTestUserContext(req.Context(), userID, "notifuser", false))
	w := httptest.NewRecorder()

	handler.GetNotifications(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.GetNotificationsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Notifications) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(response.Notifications))
	}

	if response.Notifications[0].ID != notificationID3 || response.Notifications[1].ID != notificationID2 {
		t.Errorf("unexpected notification order: got %s then %s", response.Notifications[0].ID, response.Notifications[1].ID)
	}

	if response.Meta.UnreadCount != 2 {
		t.Errorf("expected unread count 2, got %d", response.Meta.UnreadCount)
	}

	if response.Meta.Cursor == nil || !response.Meta.HasMore {
		t.Fatalf("expected cursor and has_more for pagination")
	}

	cursor := url.QueryEscape(*response.Meta.Cursor)
	req = httptest.NewRequest("GET", "/api/v1/notifications?limit=2&cursor="+cursor, nil)
	req = req.WithContext(createTestUserContext(req.Context(), userID, "notifuser", false))
	w = httptest.NewRecorder()

	handler.GetNotifications(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var nextResponse models.GetNotificationsResponse
	if err := json.NewDecoder(w.Body).Decode(&nextResponse); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(nextResponse.Notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(nextResponse.Notifications))
	}

	if nextResponse.Notifications[0].ID != notificationID1 {
		t.Errorf("expected notification %s, got %s", notificationID1, nextResponse.Notifications[0].ID)
	}

	if nextResponse.Meta.HasMore {
		t.Error("expected has_more false on final page")
	}

	if nextResponse.Meta.Cursor != nil {
		t.Error("expected no cursor on final page")
	}
}

func TestGetNotificationsInvalidMethod(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewNotificationHandler(db, nil)
	req := httptest.NewRequest("POST", "/api/v1/notifications", nil)
	w := httptest.NewRecorder()

	handler.GetNotifications(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestGetNotificationsInvalidCursor(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "notifcursoruser", "notifcursoruser@test.com", false, true))
	handler := NewNotificationHandler(db, nil)

	req := httptest.NewRequest("GET", "/api/v1/notifications?cursor=not-a-cursor", nil)
	req = req.WithContext(createTestUserContext(req.Context(), userID, "notifcursoruser", false))
	w := httptest.NewRecorder()

	handler.GetNotifications(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if response.Code != "INVALID_CURSOR" {
		t.Errorf("expected error code INVALID_CURSOR, got %s", response.Code)
	}
}

func TestGetNotificationsCursorNotFound(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "notifmissinguser", "notifmissinguser@test.com", false, true))
	handler := NewNotificationHandler(db, nil)

	req := httptest.NewRequest("GET", "/api/v1/notifications?cursor="+uuid.New().String(), nil)
	req = req.WithContext(createTestUserContext(req.Context(), userID, "notifmissinguser", false))
	w := httptest.NewRecorder()

	handler.GetNotifications(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if response.Code != "CURSOR_NOT_FOUND" {
		t.Errorf("expected error code CURSOR_NOT_FOUND, got %s", response.Code)
	}
}

func TestGetNotificationsUnauthorized(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewNotificationHandler(db, nil)
	req := httptest.NewRequest("GET", "/api/v1/notifications", nil)
	w := httptest.NewRecorder()

	handler.GetNotifications(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestMarkNotificationReadSuccess(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "notifreaduser", "notifreaduser@test.com", false, true))
	handler := NewNotificationHandler(db, nil)

	notificationID := uuid.New()
	insertTestNotification(t, db, notificationID, userID, time.Now().UTC().Add(-10*time.Minute), nil)

	req := httptest.NewRequest("PATCH", "/api/v1/notifications/"+notificationID.String(), nil)
	req = req.WithContext(createTestUserContext(req.Context(), userID, "notifreaduser", false))
	w := httptest.NewRecorder()

	handler.MarkNotificationRead(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response models.UpdateNotificationResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Notification.ID != notificationID {
		t.Errorf("expected notification id %s, got %s", notificationID, response.Notification.ID)
	}

	if response.Notification.ReadAt == nil {
		t.Error("expected read_at to be set")
	}
}

func TestMarkNotificationReadInvalidMethod(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	handler := NewNotificationHandler(db, nil)
	req := httptest.NewRequest("GET", "/api/v1/notifications/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	handler.MarkNotificationRead(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestMarkNotificationReadInvalidID(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "notifinvaliduser", "notifinvaliduser@test.com", false, true))
	handler := NewNotificationHandler(db, nil)

	req := httptest.NewRequest("PATCH", "/api/v1/notifications/not-a-uuid", nil)
	req = req.WithContext(createTestUserContext(req.Context(), userID, "notifinvaliduser", false))
	w := httptest.NewRecorder()

	handler.MarkNotificationRead(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if response.Code != "INVALID_NOTIFICATION_ID" {
		t.Errorf("expected error code INVALID_NOTIFICATION_ID, got %s", response.Code)
	}
}

func TestMarkNotificationReadNotFound(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	userID := uuid.MustParse(testutil.CreateTestUser(t, db, "notifmissingread", "notifmissingread@test.com", false, true))
	handler := NewNotificationHandler(db, nil)

	req := httptest.NewRequest("PATCH", "/api/v1/notifications/"+uuid.New().String(), nil)
	req = req.WithContext(createTestUserContext(req.Context(), userID, "notifmissingread", false))
	w := httptest.NewRecorder()

	handler.MarkNotificationRead(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if response.Code != "NOTIFICATION_NOT_FOUND" {
		t.Errorf("expected error code NOTIFICATION_NOT_FOUND, got %s", response.Code)
	}
}

func TestMarkNotificationReadForbidden(t *testing.T) {
	db := testutil.RequireTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	ownerID := uuid.MustParse(testutil.CreateTestUser(t, db, "notifowner", "notifowner@test.com", false, true))
	otherUserID := uuid.MustParse(testutil.CreateTestUser(t, db, "notifother", "notifother@test.com", false, true))
	handler := NewNotificationHandler(db, nil)

	notificationID := uuid.New()
	insertTestNotification(t, db, notificationID, ownerID, time.Now().UTC().Add(-5*time.Minute), nil)

	req := httptest.NewRequest("PATCH", "/api/v1/notifications/"+notificationID.String(), nil)
	req = req.WithContext(createTestUserContext(req.Context(), otherUserID, "notifother", false))
	w := httptest.NewRecorder()

	handler.MarkNotificationRead(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}

	var response models.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if response.Code != "FORBIDDEN" {
		t.Errorf("expected error code FORBIDDEN, got %s", response.Code)
	}
}
