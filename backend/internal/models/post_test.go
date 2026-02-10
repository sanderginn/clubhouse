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
			err := ValidateHighlights(tt.sectionType, tt.highlights)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatePodcastMetadata(t *testing.T) {
	validShow := &PodcastMetadata{
		Kind: "show",
		HighlightEpisodes: []PodcastHighlightEpisode{
			{
				Title: strings.Repeat("a", maxPodcastHighlightEpisodeTitleSize),
				URL:   "https://example.com/episodes/1",
			},
		},
	}
	validEpisode := &PodcastMetadata{Kind: "episode"}

	tests := []struct {
		name        string
		sectionType string
		podcast     *PodcastMetadata
		wantErr     bool
	}{
		{
			name:        "nil metadata allowed",
			sectionType: "general",
			podcast:     nil,
			wantErr:     false,
		},
		{
			name:        "podcast metadata not allowed in non podcast section",
			sectionType: "music",
			podcast:     validShow,
			wantErr:     true,
		},
		{
			name:        "kind required",
			sectionType: "podcast",
			podcast:     &PodcastMetadata{},
			wantErr:     true,
		},
		{
			name:        "kind must be show or episode",
			sectionType: "podcast",
			podcast:     &PodcastMetadata{Kind: "series"},
			wantErr:     true,
		},
		{
			name:        "episode cannot include highlights",
			sectionType: "podcast",
			podcast: &PodcastMetadata{
				Kind: "episode",
				HighlightEpisodes: []PodcastHighlightEpisode{
					{Title: "Episode 1", URL: "https://example.com/episodes/1"},
				},
			},
			wantErr: true,
		},
		{
			name:        "too many highlighted episodes",
			sectionType: "podcast",
			podcast: &PodcastMetadata{
				Kind:              "show",
				HighlightEpisodes: make([]PodcastHighlightEpisode, maxPodcastHighlightEpisodesPerLink+1),
			},
			wantErr: true,
		},
		{
			name:        "highlight title required",
			sectionType: "podcast",
			podcast: &PodcastMetadata{
				Kind: "show",
				HighlightEpisodes: []PodcastHighlightEpisode{
					{Title: " ", URL: "https://example.com/episodes/1"},
				},
			},
			wantErr: true,
		},
		{
			name:        "highlight title too long",
			sectionType: "podcast",
			podcast: &PodcastMetadata{
				Kind: "show",
				HighlightEpisodes: []PodcastHighlightEpisode{
					{Title: strings.Repeat("b", maxPodcastHighlightEpisodeTitleSize+1), URL: "https://example.com/episodes/1"},
				},
			},
			wantErr: true,
		},
		{
			name:        "highlight url required",
			sectionType: "podcast",
			podcast: &PodcastMetadata{
				Kind: "show",
				HighlightEpisodes: []PodcastHighlightEpisode{
					{Title: "Episode 1", URL: " "},
				},
			},
			wantErr: true,
		},
		{
			name:        "highlight url too long",
			sectionType: "podcast",
			podcast: &PodcastMetadata{
				Kind: "show",
				HighlightEpisodes: []PodcastHighlightEpisode{
					{Title: "Episode 1", URL: "https://" + strings.Repeat("x", 2050)},
				},
			},
			wantErr: true,
		},
		{
			name:        "highlight url must be http or https",
			sectionType: "podcast",
			podcast: &PodcastMetadata{
				Kind: "show",
				HighlightEpisodes: []PodcastHighlightEpisode{
					{Title: "Episode 1", URL: "ftp://example.com/episodes/1"},
				},
			},
			wantErr: true,
		},
		{
			name:        "highlight note too long",
			sectionType: "podcast",
			podcast: &PodcastMetadata{
				Kind: "show",
				HighlightEpisodes: []PodcastHighlightEpisode{
					{
						Title: "Episode 1",
						URL:   "https://example.com/episodes/1",
						Note:  func() *string { value := strings.Repeat("n", maxPodcastHighlightEpisodeNoteSize+1); return &value }(),
					},
				},
			},
			wantErr: true,
		},
		{
			name:        "valid show metadata",
			sectionType: "podcast",
			podcast:     validShow,
			wantErr:     false,
		},
		{
			name:        "valid episode metadata",
			sectionType: "podcast",
			podcast:     validEpisode,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePodcastMetadata(tt.sectionType, tt.podcast)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
