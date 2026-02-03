package links

import (
	"context"
	"testing"
)

func TestYouTubeExtractorCanExtract(t *testing.T) {
	extractor := YouTubeExtractor{}

	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", true},
		{"https://youtu.be/dQw4w9WgXcQ", true},
		{"https://www.youtube.com/embed/dQw4w9WgXcQ", true},
		{"https://example.com/watch?v=dQw4w9WgXcQ", false},
	}

	for _, test := range cases {
		if got := extractor.CanExtract(test.url); got != test.want {
			t.Errorf("CanExtract(%q) = %v, want %v", test.url, got, test.want)
		}
	}
}

func TestYouTubeExtractorExtract(t *testing.T) {
	extractor := YouTubeExtractor{}

	cases := []struct {
		url         string
		expectID    string
		expectError bool
	}{
		{
			url:      "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			expectID: "dQw4w9WgXcQ",
		},
		{
			url:      "https://youtu.be/dQw4w9WgXcQ",
			expectID: "dQw4w9WgXcQ",
		},
		{
			url:      "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42s",
			expectID: "dQw4w9WgXcQ",
		},
		{
			url:      "https://www.youtube.com/embed/dQw4w9WgXcQ?start=10",
			expectID: "dQw4w9WgXcQ",
		},
		{
			url:         "https://www.youtube.com/watch?v=",
			expectError: true,
		},
	}

	for _, test := range cases {
		embed, err := extractor.Extract(context.Background(), test.url)
		if test.expectError {
			if err == nil {
				t.Fatalf("expected error for %q", test.url)
			}
			continue
		}
		if err != nil {
			t.Fatalf("Extract(%q) error: %v", test.url, err)
		}
		if embed == nil {
			t.Fatalf("expected embed for %q", test.url)
		}
		expectedURL := youtubeEmbedBaseURL + test.expectID
		if embed.EmbedURL != expectedURL {
			t.Fatalf("EmbedURL = %q, want %q", embed.EmbedURL, expectedURL)
		}
		if embed.Provider != "youtube" {
			t.Fatalf("Provider = %q, want youtube", embed.Provider)
		}
		if embed.Type != "iframe" {
			t.Fatalf("Type = %q, want iframe", embed.Type)
		}
	}
}
