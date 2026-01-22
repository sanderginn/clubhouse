package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/services"
)

func TestGetVAPIDKey(t *testing.T) {
	db, _, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("failed to setup mock db: %v", err)
	}
	defer db.Close()

	handler := NewPushHandler(db, services.NewPushService(db))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/push/vapid-key", nil)
	req = req.WithContext(createTestUserContext(req.Context(), uuid.New(), "testuser", false))
	rr := httptest.NewRecorder()

	handler.GetVAPIDKey(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var response models.PushVAPIDKeyResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.PublicKey == "" {
		t.Fatalf("expected public key to be set")
	}
}

func TestSubscribePushSuccess(t *testing.T) {
	db, mock, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("failed to setup mock db: %v", err)
	}
	defer db.Close()

	handler := NewPushHandler(db, services.NewPushService(db))
	userID := uuid.New()

	reqBody, _ := json.Marshal(models.PushSubscriptionRequest{
		Endpoint: "https://example.com/endpoint",
		Keys: models.PushSubscriptionKeys{
			Auth:   "auth-key",
			P256dh: "p256dh-key",
		},
	})

	mock.ExpectExec("INSERT INTO push_subscriptions").
		WithArgs(userID, "https://example.com/endpoint", "auth-key", "p256dh-key").
		WillReturnResult(sqlmock.NewResult(1, 1))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/push/subscribe", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(createTestUserContext(req.Context(), userID, "testuser", false))

	rr := httptest.NewRecorder()
	handler.Subscribe(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rr.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}

func TestSubscribePushInvalidBody(t *testing.T) {
	db, _, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("failed to setup mock db: %v", err)
	}
	defer db.Close()

	handler := NewPushHandler(db, services.NewPushService(db))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/push/subscribe", bytes.NewBufferString("{"))
	req = req.WithContext(createTestUserContext(req.Context(), uuid.New(), "testuser", false))

	rr := httptest.NewRecorder()
	handler.Subscribe(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestUnsubscribePushSuccess(t *testing.T) {
	db, mock, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("failed to setup mock db: %v", err)
	}
	defer db.Close()

	handler := NewPushHandler(db, services.NewPushService(db))
	userID := uuid.New()

	mock.ExpectExec("UPDATE push_subscriptions").
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/push/subscribe", nil)
	req = req.WithContext(createTestUserContext(req.Context(), userID, "testuser", false))

	rr := httptest.NewRecorder()
	handler.Unsubscribe(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rr.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled expectations: %v", err)
	}
}
