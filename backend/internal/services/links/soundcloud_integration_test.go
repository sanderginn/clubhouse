package links

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestSoundCloudExtractorIntegration(t *testing.T) {
	if os.Getenv("SOUNDCLOUD_OEMBED_INTEGRATION") == "" {
		t.Skip("set SOUNDCLOUD_OEMBED_INTEGRATION=1 to run")
	}

	extractor := NewSoundCloudExtractor(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	embed, err := extractor.Extract(ctx, "https://soundcloud.com/hamdiofficialmusic/counting")
	if err != nil {
		t.Fatalf("Extract error: %v", err)
	}
	if embed == nil || embed.EmbedURL == "" {
		t.Fatal("expected embed URL")
	}
}
