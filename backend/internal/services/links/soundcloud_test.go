package links

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSoundCloudExtractorCanExtract(t *testing.T) {
	extractor := NewSoundCloudExtractor(nil)

	cases := []struct {
		name string
		url  string
		want bool
	}{
		{name: "track", url: "https://soundcloud.com/artist/track", want: true},
		{name: "playlist", url: "https://soundcloud.com/artist/sets/playlist", want: true},
		{name: "subdomain", url: "https://m.soundcloud.com/artist/track", want: true},
		{name: "youtube", url: "https://youtube.com/watch?v=abc", want: false},
		{name: "invalid", url: "://bad-url", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractor.CanExtract(tc.url); got != tc.want {
				t.Fatalf("CanExtract(%q) = %v, want %v", tc.url, got, tc.want)
			}
		})
	}
}

func TestSoundCloudExtractorExtract(t *testing.T) {
	const targetURL = "https://soundcloud.com/artist/track"
	const embedSrc = "https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/tracks/123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("format") != "json" {
			t.Fatalf("format = %q, want json", query.Get("format"))
		}
		if query.Get("url") != targetURL {
			t.Fatalf("url = %q, want %q", query.Get("url"), targetURL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"type": "rich",
			"version": "1.0",
			"title": "Test Track",
			"author_name": "Artist",
			"html": "<iframe src=\"`+embedSrc+`\"></iframe>",
			"width": 100,
			"height": 166,
			"thumbnail_url": "https://example.com/image.png"
		}`)
	}))
	defer server.Close()

	extractor := NewSoundCloudExtractor(&http.Client{Timeout: time.Second})
	extractor.oEmbedURL = server.URL

	embed, err := extractor.Extract(context.Background(), targetURL)
	if err != nil {
		t.Fatalf("Extract error: %v", err)
	}
	if embed == nil {
		t.Fatal("expected embed data")
	}
	if embed.Provider != "soundcloud" {
		t.Fatalf("provider = %q, want soundcloud", embed.Provider)
	}
	if embed.EmbedURL != embedSrc {
		t.Fatalf("embed_url = %q, want %q", embed.EmbedURL, embedSrc)
	}
	if embed.Height != 166 {
		t.Fatalf("height = %d, want 166", embed.Height)
	}
}

func TestSoundCloudExtractorExtractHandlesStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	extractor := NewSoundCloudExtractor(&http.Client{Timeout: time.Second})
	extractor.oEmbedURL = server.URL

	_, err := extractor.Extract(context.Background(), "https://soundcloud.com/artist/track")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestSoundCloudExtractorExtractMissingIframe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"html":"<div>No iframe</div>"}`)
	}))
	defer server.Close()

	extractor := NewSoundCloudExtractor(&http.Client{Timeout: time.Second})
	extractor.oEmbedURL = server.URL

	_, err := extractor.Extract(context.Background(), "https://soundcloud.com/artist/track")
	if err == nil {
		t.Fatal("expected error when iframe src missing")
	}
}
