package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"

	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
)

type pushConfig struct {
	publicKey  string
	privateKey string
	subject    string
	enabled    bool
}

var (
	pushConfigOnce sync.Once
	pushConfigData pushConfig
)

// PushService manages web push subscriptions and delivery.
type PushService struct {
	db *sql.DB
}

// PushDeliveryResult captures delivery outcomes for a push send attempt.
type PushDeliveryResult struct {
	Delivered    int64
	FailedByType map[string]int64
}

// PushDeliveryError carries a coarse error classification for push delivery failures.
type PushDeliveryError struct {
	Type string
	Err  error
}

func (e *PushDeliveryError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Type
}

func (e *PushDeliveryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func pushFailureTypeForStatus(statusCode int) (string, bool) {
	switch {
	case statusCode == http.StatusGone || statusCode == http.StatusNotFound:
		return "subscription_gone", true
	case statusCode >= http.StatusBadRequest:
		return "http_error", true
	default:
		return "", false
	}
}

func recordPushFailure(result *PushDeliveryResult, failureType string) {
	if result == nil || strings.TrimSpace(failureType) == "" {
		return
	}
	if result.FailedByType == nil {
		result.FailedByType = make(map[string]int64)
	}
	result.FailedByType[failureType]++
}

// NewPushService creates a push service with shared VAPID config.
func NewPushService(db *sql.DB) *PushService {
	pushConfigOnce.Do(func() {
		publicKey := strings.TrimSpace(os.Getenv("VAPID_PUBLIC_KEY"))
		privateKey := strings.TrimSpace(os.Getenv("VAPID_PRIVATE_KEY"))
		subject := strings.TrimSpace(os.Getenv("VAPID_SUBJECT"))
		if subject == "" {
			subject = "mailto:admin@clubhouse.local"
		}

		switch {
		case publicKey == "" && privateKey == "":
			generatedPrivate, generatedPublic, err := webpush.GenerateVAPIDKeys()
			if err != nil {
				observability.LogError(context.Background(), observability.ErrorLog{
					Message:    "failed to generate VAPID keys",
					Code:       "VAPID_GENERATE_FAILED",
					StatusCode: http.StatusInternalServerError,
					Err:        err,
				})
				pushConfigData = pushConfig{subject: subject, enabled: false}
				return
			}
			observability.LogInfo(context.Background(), "generated ephemeral VAPID keys; set VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY to persist")
			pushConfigData = pushConfig{
				publicKey:  generatedPublic,
				privateKey: generatedPrivate,
				subject:    subject,
				enabled:    true,
			}
		case publicKey == "" || privateKey == "":
			observability.LogError(context.Background(), observability.ErrorLog{
				Message:    "VAPID keys are partially configured; push notifications disabled",
				Code:       "VAPID_CONFIG_INVALID",
				StatusCode: http.StatusInternalServerError,
			})
			pushConfigData = pushConfig{publicKey: publicKey, privateKey: privateKey, subject: subject, enabled: false}
		default:
			pushConfigData = pushConfig{publicKey: publicKey, privateKey: privateKey, subject: subject, enabled: true}
		}
	})

	return &PushService{db: db}
}

// PublicKey returns the configured VAPID public key.
func (s *PushService) PublicKey() (string, error) {
	if strings.TrimSpace(pushConfigData.publicKey) == "" {
		return "", errors.New("vapid public key not configured")
	}
	if !pushConfigData.enabled {
		return "", errors.New("vapid keys incomplete")
	}
	return pushConfigData.publicKey, nil
}

// UpsertSubscription stores or refreshes a push subscription for a user.
func (s *PushService) UpsertSubscription(ctx context.Context, userID uuid.UUID, sub models.PushSubscriptionRequest) error {
	query := `
		INSERT INTO push_subscriptions (user_id, endpoint, auth_key, p256dh_key)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (endpoint)
		DO UPDATE SET
			user_id = EXCLUDED.user_id,
			auth_key = EXCLUDED.auth_key,
			p256dh_key = EXCLUDED.p256dh_key,
			deleted_at = NULL
	`

	_, err := s.db.ExecContext(ctx, query, userID, sub.Endpoint, sub.Keys.Auth, sub.Keys.P256dh)
	if err != nil {
		return err
	}

	return nil
}

// DeleteSubscriptions removes all active subscriptions for a user.
func (s *PushService) DeleteSubscriptions(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE push_subscriptions
		SET deleted_at = now()
		WHERE user_id = $1 AND deleted_at IS NULL
	`, userID)
	return err
}

// SendNotification delivers a push notification to all active subscriptions for a user.
func (s *PushService) SendNotification(ctx context.Context, userID uuid.UUID, payload models.PushNotificationPayload) (PushDeliveryResult, error) {
	result := PushDeliveryResult{}
	if !pushConfigData.enabled {
		return result, nil
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		recordPushFailure(&result, "payload_error")
		return result, &PushDeliveryError{Type: "payload_error", Err: err}
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT endpoint, auth_key, p256dh_key
		FROM push_subscriptions
		WHERE user_id = $1 AND deleted_at IS NULL
	`, userID)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	var sendErr error
	for rows.Next() {
		var endpoint string
		var authKey string
		var p256dhKey string
		if err := rows.Scan(&endpoint, &authKey, &p256dhKey); err != nil {
			return result, err
		}

		subscription := &webpush.Subscription{
			Endpoint: endpoint,
			Keys: webpush.Keys{
				Auth:   authKey,
				P256dh: p256dhKey,
			},
		}

		resp, err := webpush.SendNotification(payloadBytes, subscription, &webpush.Options{
			Subscriber:      pushConfigData.subject,
			VAPIDPublicKey:  pushConfigData.publicKey,
			VAPIDPrivateKey: pushConfigData.privateKey,
			TTL:             60,
		})
		if err != nil {
			recordPushFailure(&result, "send_error")
			if sendErr == nil {
				sendErr = &PushDeliveryError{Type: "send_error", Err: err}
			}
			continue
		}
		if resp != nil {
			if failureType, isFailure := pushFailureTypeForStatus(resp.StatusCode); isFailure {
				recordPushFailure(&result, failureType)
				if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
					_ = s.markSubscriptionDeleted(ctx, endpoint)
				} else if sendErr == nil {
					sendErr = &PushDeliveryError{Type: failureType, Err: errors.New(resp.Status)}
				}
			} else {
				result.Delivered++
			}
			_ = resp.Body.Close()
		}
	}

	if err := rows.Err(); err != nil {
		return result, err
	}

	return result, sendErr
}

// SendNotificationToUsers delivers the same push payload to multiple users.
func (s *PushService) SendNotificationToUsers(ctx context.Context, userIDs []uuid.UUID, payload models.PushNotificationPayload) (PushDeliveryResult, error) {
	result := PushDeliveryResult{}
	var sendErr error
	for _, userID := range userIDs {
		userResult, err := s.SendNotification(ctx, userID, payload)
		result.Delivered += userResult.Delivered
		for failureType, count := range userResult.FailedByType {
			if result.FailedByType == nil {
				result.FailedByType = make(map[string]int64)
			}
			result.FailedByType[failureType] += count
		}
		if err != nil && sendErr == nil {
			sendErr = err
		}
	}
	return result, sendErr
}

func (s *PushService) markSubscriptionDeleted(ctx context.Context, endpoint string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE push_subscriptions
		SET deleted_at = now()
		WHERE endpoint = $1 AND deleted_at IS NULL
	`, endpoint)
	return err
}
