package models

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type highlightIDPayload struct {
	LinkID    string `json:"link_id"`
	Timestamp int    `json:"timestamp"`
	Label     string `json:"label,omitempty"`
}

// EncodeHighlightID builds a stable, URL-safe identifier for a highlight.
func EncodeHighlightID(linkID uuid.UUID, highlight Highlight) (string, error) {
	payload := highlightIDPayload{
		LinkID:    linkID.String(),
		Timestamp: highlight.Timestamp,
		Label:     highlight.Label,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to encode highlight id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

// DecodeHighlightID parses a highlight identifier into link ID and highlight data.
func DecodeHighlightID(value string) (uuid.UUID, Highlight, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return uuid.UUID{}, Highlight{}, fmt.Errorf("invalid highlight id")
	}
	var payload highlightIDPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return uuid.UUID{}, Highlight{}, fmt.Errorf("invalid highlight id")
	}
	linkID, err := uuid.Parse(payload.LinkID)
	if err != nil {
		return uuid.UUID{}, Highlight{}, fmt.Errorf("invalid highlight id")
	}
	highlight := Highlight{
		Timestamp: payload.Timestamp,
		Label:     payload.Label,
	}
	return linkID, highlight, nil
}
