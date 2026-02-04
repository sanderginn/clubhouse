package links

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestBandcampAzureTLSIntegration(t *testing.T) {
	if os.Getenv("BANDCAMP_INTEGRATION") != "1" {
		t.Skip("set BANDCAMP_INTEGRATION=1 to run")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Reset to use real fetch function
	SetBandcampFetchHTMLForTests(nil)

	metadata, err := FetchMetadata(ctx, "https://powersnatch.bandcamp.com/album/ep1")
	if err != nil {
		t.Fatalf("FetchMetadata error: %v", err)
	}

	if metadata["provider"] != "bandcamp" {
		t.Errorf("provider = %v, want bandcamp", metadata["provider"])
	}
	if metadata["title"] == nil || metadata["title"] == "" {
		t.Error("expected title to be present")
	}
	if metadata["image"] == nil || metadata["image"] == "" {
		t.Error("expected image to be present")
	}

	embed, ok := metadata["embed"].(*EmbedData)
	if !ok || embed == nil {
		t.Fatal("expected embed to be present")
	}
	if embed.Provider != "bandcamp" {
		t.Errorf("embed provider = %v, want bandcamp", embed.Provider)
	}
	if embed.EmbedURL == "" {
		t.Error("expected embed URL to be present")
	}

	t.Logf("Title: %v", metadata["title"])
	t.Logf("Image: %v", metadata["image"])
	t.Logf("Embed URL: %s", embed.EmbedURL)
}
