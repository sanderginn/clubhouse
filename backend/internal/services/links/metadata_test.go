package links

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchMetadataHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
			<html>
			<head>
				<title>Fallback Title</title>
				<meta property="og:title" content="OG Title" />
				<meta name="description" content="Desc" />
				<meta property="og:image" content="/img.png" />
				<meta property="og:site_name" content="ExampleSite" />
			</head>
			</html>`))
	}))
	defer server.Close()

	metadata, err := FetchMetadata(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("FetchMetadata error: %v", err)
	}

	if metadata["title"] != "OG Title" {
		t.Errorf("title = %v, want OG Title", metadata["title"])
	}
	if metadata["description"] != "Desc" {
		t.Errorf("description = %v, want Desc", metadata["description"])
	}
	if provider := metadata["provider"]; provider != "ExampleSite" {
		t.Errorf("provider = %v, want ExampleSite", provider)
	}
	image, _ := metadata["image"].(string)
	if !strings.HasPrefix(image, server.URL) {
		t.Errorf("image = %q, want prefix %q", image, server.URL)
	}
}

func TestFetchMetadataTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html></html>"))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	fetcher := NewFetcher(&http.Client{})
	_, err := fetcher.Fetch(ctx, server.URL)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestDetectProvider(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{host: "open.spotify.com", want: "spotify"},
		{host: "www.youtube.com", want: "youtube"},
		{host: "youtu.be", want: "youtube"},
		{host: "imdb.com", want: "imdb"},
	}

	for _, tt := range tests {
		if got := detectProvider(tt.host); got != tt.want {
			t.Errorf("detectProvider(%q) = %q, want %q", tt.host, got, tt.want)
		}
	}
}
