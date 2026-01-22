package models

import "github.com/google/uuid"

// PushSubscriptionKeys represents the keys for a push subscription.
type PushSubscriptionKeys struct {
	Auth   string `json:"auth"`
	P256dh string `json:"p256dh"`
}

// PushSubscriptionRequest represents a push subscription payload from the browser.
type PushSubscriptionRequest struct {
	Endpoint string               `json:"endpoint"`
	Keys     PushSubscriptionKeys `json:"keys"`
}

// PushVAPIDKeyResponse represents the response for the VAPID public key endpoint.
type PushVAPIDKeyResponse struct {
	PublicKey string `json:"publicKey"`
}

// PushNotificationPayload is sent to clients via Web Push.
type PushNotificationPayload struct {
	Title         string     `json:"title,omitempty"`
	Body          string     `json:"body,omitempty"`
	Type          string     `json:"type"`
	PostID        *uuid.UUID `json:"post_id,omitempty"`
	CommentID     *uuid.UUID `json:"comment_id,omitempty"`
	RelatedUserID *uuid.UUID `json:"related_user_id,omitempty"`
}
