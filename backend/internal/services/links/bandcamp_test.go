package links

import (
	"context"
	"strings"
	"testing"
)

func TestBandcampExtractorCanExtract(t *testing.T) {
	extractor := BandcampExtractor{}

	if !extractor.CanExtract("https://artist.bandcamp.com/album/test-album") {
		t.Fatalf("expected bandcamp url to be extractable")
	}
	if extractor.CanExtract("https://example.com/album/test") {
		t.Fatalf("expected non-bandcamp url to be ignored")
	}
}

func TestBandcampExtractorExtractFromHTMLAlbum(t *testing.T) {
	extractor := BandcampExtractor{}
	metaTags := map[string]string{
		"bc-page-properties": `{"item_type":"a","item_id":12345}`,
	}

	embed, err := extractor.ExtractFromHTML(
		context.Background(),
		"https://artist.bandcamp.com/album/test-album",
		nil,
		metaTags,
	)
	if err != nil {
		t.Fatalf("expected embed, got error: %v", err)
	}
	if embed.Provider != "bandcamp" {
		t.Fatalf("provider = %v, want bandcamp", embed.Provider)
	}
	if embed.Height != bandcampAlbumHeight {
		t.Fatalf("height = %v, want %d", embed.Height, bandcampAlbumHeight)
	}
	if embed.EmbedURL == "" || embed.Type != "iframe" {
		t.Fatalf("expected iframe embed url to be set")
	}
}

func TestBandcampExtractorExtractFromHTMLEscapedTrack(t *testing.T) {
	extractor := BandcampExtractor{}
	metaTags := map[string]string{
		"bc-page-properties": "{&quot;item_type&quot;:&quot;t&quot;,&quot;item_id&quot;:67890}",
	}

	embed, err := extractor.ExtractFromHTML(
		context.Background(),
		"https://artist.bandcamp.com/track/test-track",
		nil,
		metaTags,
	)
	if err != nil {
		t.Fatalf("expected embed, got error: %v", err)
	}
	if embed.Height != bandcampTrackHeight {
		t.Fatalf("height = %v, want %d", embed.Height, bandcampTrackHeight)
	}
}

func TestExtractBandcampFromJSONLDAlbum(t *testing.T) {
	html := []byte(`
<!DOCTYPE html>
<html>
<head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "MusicAlbum",
  "name": "Test Album",
  "byArtist": {
    "@type": "MusicGroup",
    "name": "Test Artist"
  },
  "image": "https://f4.bcbits.com/img/a123456_10.jpg",
  "albumRelease": [{
    "@type": "MusicRelease",
    "additionalProperty": [{
      "@type": "PropertyValue",
      "name": "item_id",
      "value": 987654321
    }]
  }]
}
</script>
</head>
</html>
`)

	content, metadata, err := extractBandcampFromJSONLD(html)
	if err != nil {
		t.Fatalf("expected JSON-LD content, got error: %v", err)
	}
	if content.Type != "album" {
		t.Fatalf("type = %q, want album", content.Type)
	}
	if content.ID != "987654321" {
		t.Fatalf("id = %q, want 987654321", content.ID)
	}
	if metadata["title"] != "Test Album" {
		t.Fatalf("title = %v, want Test Album", metadata["title"])
	}
	if metadata["artist"] != "Test Artist" {
		t.Fatalf("artist = %v, want Test Artist", metadata["artist"])
	}
	if metadata["image"] != "https://f4.bcbits.com/img/a123456_10.jpg" {
		t.Fatalf("image = %v, want json-ld image", metadata["image"])
	}
}

func TestExtractBandcampFromJSONLDTrack(t *testing.T) {
	html := []byte(`
<!DOCTYPE html>
<html>
<head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "MusicRecording",
  "name": "Test Track",
  "byArtist": {
    "@type": "MusicGroup",
    "name": "Test Artist"
  },
  "image": "https://f4.bcbits.com/img/a123456_10.jpg",
  "inAlbum": {
    "@type": "MusicAlbum",
    "additionalProperty": [{
      "@type": "PropertyValue",
      "name": "item_id",
      "value": "111222333"
    }]
  }
}
</script>
</head>
</html>
`)

	content, metadata, err := extractBandcampFromJSONLD(html)
	if err != nil {
		t.Fatalf("expected JSON-LD content, got error: %v", err)
	}
	if content.Type != "track" {
		t.Fatalf("type = %q, want track", content.Type)
	}
	if content.ID != "111222333" {
		t.Fatalf("id = %q, want 111222333", content.ID)
	}
	if metadata["title"] != "Test Track" {
		t.Fatalf("title = %v, want Test Track", metadata["title"])
	}
}

func TestBandcampExtractorExtractFromHTMLFallsBackToPageProperties(t *testing.T) {
	extractor := BandcampExtractor{}
	body := []byte(`
<!DOCTYPE html>
<html>
<body>
<div data-bc-page-properties='{"item_type":"album","item_id":123456}'></div>
</body>
</html>
`)

	embed, err := extractor.ExtractFromHTML(
		context.Background(),
		"https://artist.bandcamp.com/album/test-album",
		body,
		nil,
	)
	if err != nil {
		t.Fatalf("expected embed, got error: %v", err)
	}
	if embed.Provider != "bandcamp" {
		t.Fatalf("provider = %v, want bandcamp", embed.Provider)
	}
	if embed.EmbedURL == "" || embed.Type != "iframe" {
		t.Fatalf("expected iframe embed url to be set")
	}
	if embed.Height != bandcampAlbumHeight {
		t.Fatalf("height = %v, want %d", embed.Height, bandcampAlbumHeight)
	}
	if !strings.Contains(embed.EmbedURL, "album=123456") {
		t.Fatalf("embed url = %v, want album=123456", embed.EmbedURL)
	}
}
