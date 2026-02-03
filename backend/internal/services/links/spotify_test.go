package links

import (
	"context"
	"testing"
)

func TestParseSpotifyURL(t *testing.T) {
	cases := []struct {
		name       string
		url        string
		wantType   string
		wantID     string
		shouldFail bool
	}{
		{
			name:     "track",
			url:      "https://open.spotify.com/track/3n3Ppam7vgaVa1iaRUc9Lp",
			wantType: "track",
			wantID:   "3n3Ppam7vgaVa1iaRUc9Lp",
		},
		{
			name:     "album",
			url:      "https://open.spotify.com/album/6mUdeDZCsExyJLMdAfDuwh",
			wantType: "album",
			wantID:   "6mUdeDZCsExyJLMdAfDuwh",
		},
		{
			name:     "playlist",
			url:      "https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M",
			wantType: "playlist",
			wantID:   "37i9dQZF1DXcBWIGoYBM5M",
		},
		{
			name:     "artist",
			url:      "https://open.spotify.com/artist/1dfeR4HaWDbWqFHLkxsg1d",
			wantType: "artist",
			wantID:   "1dfeR4HaWDbWqFHLkxsg1d",
		},
		{
			name:     "show",
			url:      "https://open.spotify.com/show/2rN1dT9uLw8wKjtBF6qWvG",
			wantType: "show",
			wantID:   "2rN1dT9uLw8wKjtBF6qWvG",
		},
		{
			name:     "episode",
			url:      "https://open.spotify.com/episode/4rOoJ6Egrf8K2IrywzwOMk",
			wantType: "episode",
			wantID:   "4rOoJ6Egrf8K2IrywzwOMk",
		},
		{
			name:     "embed url",
			url:      "https://open.spotify.com/embed/track/3n3Ppam7vgaVa1iaRUc9Lp",
			wantType: "track",
			wantID:   "3n3Ppam7vgaVa1iaRUc9Lp",
		},
		{
			name:       "invalid host",
			url:        "https://example.com/track/3n3Ppam7vgaVa1iaRUc9Lp",
			shouldFail: true,
		},
		{
			name:       "unsupported type",
			url:        "https://open.spotify.com/user/123",
			shouldFail: true,
		},
	}

	for _, tc := range cases {
		result, err := parseSpotifyURL(tc.url)
		if tc.shouldFail {
			if err == nil {
				t.Fatalf("%s: expected error", tc.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: parse error: %v", tc.name, err)
		}
		if result.Type != tc.wantType {
			t.Errorf("%s: type = %s, want %s", tc.name, result.Type, tc.wantType)
		}
		if result.ID != tc.wantID {
			t.Errorf("%s: id = %s, want %s", tc.name, result.ID, tc.wantID)
		}
	}
}

func TestSpotifyExtractorExtract(t *testing.T) {
	extractor := SpotifyExtractor{}
	url := "https://open.spotify.com/track/3n3Ppam7vgaVa1iaRUc9Lp"

	embed, err := extractor.Extract(context.Background(), url)
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	if embed.EmbedURL != "https://open.spotify.com/embed/track/3n3Ppam7vgaVa1iaRUc9Lp" {
		t.Fatalf("embed url = %s", embed.EmbedURL)
	}
	if embed.Provider != "spotify" {
		t.Fatalf("provider = %s", embed.Provider)
	}
	if embed.Height != 152 {
		t.Fatalf("height = %d", embed.Height)
	}
}
