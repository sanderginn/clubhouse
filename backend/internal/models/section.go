package models

import "github.com/google/uuid"

type Section struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Type string    `json:"type"`
}

type ListSectionsResponse struct {
	Sections []Section `json:"sections"`
}
