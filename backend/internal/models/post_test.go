package models

import (
	"strings"
	"testing"
)

func TestValidateHighlights(t *testing.T) {
	validHighlights := make([]Highlight, maxHighlightsPerLink)
	for i := range validHighlights {
		validHighlights[i] = Highlight{
			Timestamp: i,
			Label:     strings.Repeat("a", maxHighlightLabelLength),
		}
	}

	tests := []struct {
		name        string
		sectionType string
		highlights  []Highlight
		wantErr     bool
	}{
		{
			name:        "no highlights allowed for any section",
			sectionType: "books",
			highlights:  nil,
			wantErr:     false,
		},
		{
			name:        "highlights not allowed for section type",
			sectionType: "movies",
			highlights: []Highlight{
				{Timestamp: 10, Label: "intro"},
			},
			wantErr: true,
		},
		{
			name:        "too many highlights",
			sectionType: "music",
			highlights:  append(validHighlights, Highlight{Timestamp: 999}),
			wantErr:     true,
		},
		{
			name:        "negative timestamp",
			sectionType: "music",
			highlights: []Highlight{
				{Timestamp: -1, Label: "bad"},
			},
			wantErr: true,
		},
		{
			name:        "label too long",
			sectionType: "music",
			highlights: []Highlight{
				{Timestamp: 5, Label: strings.Repeat("b", maxHighlightLabelLength+1)},
			},
			wantErr: true,
		},
		{
			name:        "valid highlights at limits",
			sectionType: "music",
			highlights:  validHighlights,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHighlights(tt.sectionType, tt.highlights)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
