package services

import "testing"

func TestDetectPodcastKindFromURLSpotifyShowAndEpisode(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantKind string
		wantOK   bool
	}{
		{
			name:     "issue example spotify show",
			url:      "https://open.spotify.com/show/4J3UybFDArcDcxJPKj0OyH",
			wantKind: "show",
			wantOK:   true,
		},
		{
			name:     "issue example spotify episode",
			url:      "https://open.spotify.com/episode/4qrnpiJaEEmHxkZy7RHh5h",
			wantKind: "episode",
			wantOK:   true,
		},
		{
			name:     "spotify localized show",
			url:      "https://open.spotify.com/intl-en/show/4J3UybFDArcDcxJPKj0OyH",
			wantKind: "show",
			wantOK:   true,
		},
		{
			name:     "spotify localized episode",
			url:      "https://open.spotify.com/intl-en/episode/4qrnpiJaEEmHxkZy7RHh5h",
			wantKind: "episode",
			wantOK:   true,
		},
		{
			name:     "spotify embed show",
			url:      "https://open.spotify.com/embed/show/4J3UybFDArcDcxJPKj0OyH",
			wantKind: "show",
			wantOK:   true,
		},
		{
			name:     "spotify embed episode",
			url:      "https://open.spotify.com/embed/episode/4qrnpiJaEEmHxkZy7RHh5h",
			wantKind: "episode",
			wantOK:   true,
		},
		{
			name:     "spotify localized embed show",
			url:      "https://open.spotify.com/intl-en/embed/show/4J3UybFDArcDcxJPKj0OyH",
			wantKind: "show",
			wantOK:   true,
		},
		{
			name:     "spotify embed localized episode",
			url:      "https://open.spotify.com/embed/intl-en/episode/4qrnpiJaEEmHxkZy7RHh5h",
			wantKind: "episode",
			wantOK:   true,
		},
		{
			name:   "spotify show missing id",
			url:    "https://open.spotify.com/show",
			wantOK: false,
		},
		{
			name:   "spotify embed episode missing id",
			url:    "https://open.spotify.com/embed/episode",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKind, ok := detectPodcastKindFromURL(tt.url)
			if ok != tt.wantOK {
				t.Fatalf("detectPodcastKindFromURL(%q) ok=%v, want %v", tt.url, ok, tt.wantOK)
			}
			if tt.wantOK && gotKind != tt.wantKind {
				t.Fatalf("detectPodcastKindFromURL(%q) kind=%q, want %q", tt.url, gotKind, tt.wantKind)
			}
		})
	}
}
