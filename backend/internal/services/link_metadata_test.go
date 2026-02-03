package services

import (
	"context"
	"testing"

	"github.com/sanderginn/clubhouse/internal/models"
	linkmeta "github.com/sanderginn/clubhouse/internal/services/links"
)

func TestFetchLinkMetadataIncludesSpotifyEmbed(t *testing.T) {
	config := GetConfigService()
	current := config.GetConfig().LinkMetadataEnabled
	enabled := true
	if _, err := config.UpdateConfig(context.Background(), &enabled, nil, nil); err != nil {
		t.Fatalf("failed to enable link metadata: %v", err)
	}
	t.Cleanup(func() {
		if _, err := config.UpdateConfig(context.Background(), &current, nil, nil); err != nil {
			t.Fatalf("failed to restore link metadata: %v", err)
		}
	})

	linkmeta.SetFetchMetadataFuncForTests(func(ctx context.Context, rawURL string) (map[string]interface{}, error) {
		return map[string]interface{}{"title": "Spotify"}, nil
	})
	t.Cleanup(func() {
		linkmeta.SetFetchMetadataFuncForTests(nil)
	})

	links := []models.LinkRequest{{URL: "https://open.spotify.com/track/3n3Ppam7vgaVa1iaRUc9Lp"}}
	metadata := fetchLinkMetadata(context.Background(), links)
	if len(metadata) != 1 {
		t.Fatalf("expected 1 metadata entry, got %d", len(metadata))
	}
	if len(metadata[0]) == 0 {
		t.Fatalf("expected metadata to be populated")
	}

	embed, ok := metadata[0]["embed"].(*linkmeta.EmbedData)
	if !ok || embed == nil {
		t.Fatalf("expected embed metadata to be present")
	}
	if embed.Provider != "spotify" {
		t.Fatalf("embed provider = %s", embed.Provider)
	}
	if embed.EmbedURL != "https://open.spotify.com/embed/track/3n3Ppam7vgaVa1iaRUc9Lp" {
		t.Fatalf("embed url = %s", embed.EmbedURL)
	}
}
